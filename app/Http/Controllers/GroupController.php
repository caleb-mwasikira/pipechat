<?php

namespace App\Http\Controllers;

use App\Mail\GroupInvite;
use App\Mail\InviteAccepted;
use App\Models\User;
use App\Support\SafeMail;
use Illuminate\Http\RedirectResponse;
use Illuminate\Http\Request;
use Carbon\Carbon;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Facades\Schema;

/*
 * Groups on the website. Only names, members and invitations live here; the
 * group's encryption key is made by the owner's terminal and passed to each new
 * member's terminal sealed to their public key (see the chat server's /api/sync).
 */
class GroupController extends Controller
{
    private const MAX_GROUPS_PER_USER = 50;

    public function index(Request $request)
    {
        $id = $request->user()->id;

        return view('groups', [
            'groups' => DB::table('chat_groups as g')
                ->join('chat_group_members as m', 'm.group_id', '=', 'g.id')
                ->where('m.user_id', $id)
                ->select('g.id', 'g.name', 'g.owner_id', 'g.gid', 'g.is_public', 'm.delivered')
                ->orderBy('g.name')->get()
                ->each(fn ($g) => $g->members = DB::table('chat_group_members as m')
                    ->join('users as u', 'u.id', '=', 'm.user_id')
                    ->where('m.group_id', $g->id)->orderBy('u.handle')->pluck('u.handle')),
            'publicGroups' => DB::table('chat_groups as g')
                ->join('users as o', 'o.id', '=', 'g.owner_id')
                ->where('g.is_public', true)
                ->whereNotIn('g.id', DB::table('chat_group_members')->where('user_id', $id)->select('group_id'))
                ->select('g.id', 'g.name', 'o.handle as owner', DB::raw('(select count(*) from chat_group_members m where m.group_id = g.id) as members'))
                ->orderBy('g.name')->limit(50)->get(),
            'invites' => DB::table('chat_group_invites as i')
                ->join('chat_groups as g', 'g.id', '=', 'i.group_id')
                ->join('users as u', 'u.id', '=', 'i.inviter_id')
                ->where('i.invitee_id', $id)->where('i.status', 'pending')
                ->select('i.id', 'g.name', 'u.handle as inviter')->get(),
        ]);
    }

    /** The page of one group: who is in it and online, what was shared, and how busy it is. */
    public function show(Request $request, int $group)
    {
        $user = $request->user();
        $row = DB::table('chat_groups')->find($group) ?? abort(404);
        abort_unless(DB::table('chat_group_members')->where('group_id', $group)->where('user_id', $user->id)->exists(), 403);

        // The chat server keeps these tables itself, so they may not exist yet.
        $hasPresence = Schema::hasTable('presence');
        $recently = now()->subSeconds(75)->timestamp;

        $members = DB::table('chat_group_members as m')
            ->join('users as u', 'u.id', '=', 'm.user_id')
            ->when($hasPresence, fn ($q) => $q->leftJoin('presence as p', 'p.handle', '=', 'u.handle')
                ->addSelect('p.online', 'p.last_seen'))
            ->where('m.group_id', $group)
            ->addSelect('u.id', 'u.handle', 'u.name', 'u.bio', 'm.delivered', 'm.created_at as joined_at')
            ->orderBy('u.handle')->get()
            ->each(function ($member) use ($row, $recently) {
                $member->is_owner = $member->id === $row->owner_id;
                $member->is_online = (bool) ($member->online ?? false) && ($member->last_seen ?? 0) >= $recently;
            })
            ->sortBy([fn ($a, $b) => $b->is_owner <=> $a->is_owner, fn ($a, $b) => $b->is_online <=> $a->is_online, fn ($a, $b) => strcmp($a->handle, $b->handle)])
            ->values();

        $messages = ['stored' => 0, 'last' => null];
        if ($row->gid && Schema::hasTable('messages')) {
            $messages['stored'] = DB::table('messages')->where('group_id', $row->gid)->count();
            $last = DB::table('messages')->where('group_id', $row->gid)->max('ts');
            $messages['last'] = $last ? Carbon::createFromTimestampMs($last) : null;
        }

        $memberIds = $members->pluck('id');

        return view('group-details', [
            'group' => $row,
            'owner' => $members->firstWhere('is_owner', true),
            'members' => $members,
            'onlineCount' => $members->where('is_online', true)->count(),
            'messages' => $messages,
            'files' => DB::table('chat_group_files as f')
                ->leftJoin('users as u', 'u.id', '=', 'f.user_id')
                ->where('f.group_id', $group)->select('f.*', 'u.handle as sharer')
                ->orderByDesc('f.id')->limit(50)->get(),
            'tracks' => DB::table('tracks as t')
                ->join('users as u', 'u.id', '=', 't.uploaded_by')
                ->whereIn('t.uploaded_by', $memberIds)->select('t.*', 'u.handle as uploader')
                ->orderByDesc('t.id')->limit(50)->get(),
            'invited' => DB::table('chat_group_invites as i')
                ->join('users as u', 'u.id', '=', 'i.invitee_id')
                ->where('i.group_id', $group)->where('i.status', 'pending')->orderBy('u.handle')->pluck('u.handle'),
            'now' => now(),
        ]);
    }

    public function store(Request $request): RedirectResponse
    {
        $data = $request->validate([
            'name' => ['required', 'string', 'max:24', 'regex:/^[^\x00-\x1f\x7f]+$/u'],
            'public' => ['nullable', 'boolean'],
        ]);
        $user = $request->user();
        $public = $request->boolean('public');

        if (DB::table('chat_group_members')->where('user_id', $user->id)->count() >= self::MAX_GROUPS_PER_USER) {
            return back()->withErrors(['name' => 'You are in too many groups.'])->withInput();
        }
        // A public group is found by its name, so the name must be free among public groups.
        if ($public && DB::table('chat_groups')->where('is_public', true)->whereRaw('lower(name) = ?', [mb_strtolower($data['name'])])->exists()) {
            return back()->withErrors(['name' => 'A public group with that name already exists. Pick another name, or make this one private.'])->withInput();
        }

        DB::transaction(function () use ($data, $user, $public) {
            $group = DB::table('chat_groups')->insertGetId([
                'name' => $data['name'], 'owner_id' => $user->id, 'is_public' => $public,
                'created_at' => now(), 'updated_at' => now(),
            ]);
            DB::table('chat_group_members')->insert([
                'group_id' => $group, 'user_id' => $user->id, 'created_at' => now(), 'updated_at' => now(),
            ]);
        });

        return back()->with('status', ($public ? 'Public group created: anyone can join it from the list. ' : 'Private group created: people need an invitation. ')
            .'It appears in your terminal the next time it syncs.');
    }

    /** Join a public group straight away, no invitation. */
    public function join(Request $request, int $group): RedirectResponse
    {
        $user = $request->user();
        $row = DB::table('chat_groups')->where('id', $group)->where('is_public', true)->first() ?? abort(404);

        if (DB::table('chat_group_members')->where('user_id', $user->id)->count() >= self::MAX_GROUPS_PER_USER) {
            return back()->withErrors(['join' => 'You are in too many groups.']);
        }
        $added = DB::table('chat_group_members')->insertOrIgnore([
            'group_id' => $row->id, 'user_id' => $user->id, 'created_at' => now(), 'updated_at' => now(),
        ]);
        if ($added && ($owner = User::find($row->owner_id))) {
            SafeMail::to($owner, new InviteAccepted($user, $row->name));
        }

        return redirect()->route('groups.show', $row->id)
            ->with('status', 'Joined. The group appears in your terminal once another member is online to share its key.');
    }

    /** The creator switches a group between private and public. */
    public function visibility(Request $request, int $group): RedirectResponse
    {
        $row = DB::table('chat_groups')->find($group) ?? abort(404);
        abort_unless($row->owner_id === $request->user()->id, 403);
        $public = $request->boolean('public');

        if ($public && DB::table('chat_groups')->where('is_public', true)->where('id', '<>', $group)->whereRaw('lower(name) = ?', [mb_strtolower($row->name)])->exists()) {
            return back()->withErrors(['visibility' => 'Another public group already has this name.']);
        }
        DB::table('chat_groups')->where('id', $group)->update(['is_public' => $public, 'updated_at' => now()]);

        return back()->with('status', $public ? 'This group is now public: anyone can join it.' : 'This group is now private. Members stay; new people need an invitation.');
    }

    public function invite(Request $request, int $group): RedirectResponse
    {
        $request->validate(['who' => ['required', 'string', 'max:255']]);
        $user = $request->user();
        abort_unless(DB::table('chat_group_members')->where('group_id', $group)->where('user_id', $user->id)->exists(), 403);

        $who = strtolower(trim(ltrim($request->input('who'), '@')));
        $invitee = User::where('handle', $who)->orWhere('email', $who)->first();
        if (! $invitee) {
            return back()->withErrors(['who' => 'No user with that handle or email.'])->withInput();
        }
        if ($invitee->id === $user->id
            || DB::table('chat_group_members')->where('group_id', $group)->where('user_id', $invitee->id)->exists()) {
            return back()->withErrors(['who' => 'They are already in this group.'])->withInput();
        }

        DB::table('chat_group_invites')->upsert([
            ['group_id' => $group, 'inviter_id' => $user->id, 'invitee_id' => $invitee->id, 'status' => 'pending',
                'created_at' => now(), 'updated_at' => now()],
        ], ['group_id', 'invitee_id'], ['inviter_id', 'status', 'updated_at']);

        SafeMail::to($invitee, new GroupInvite($user, DB::table('chat_groups')->where('id', $group)->value('name')));

        return back()->with('status', "Invitation sent to @{$invitee->handle}.");
    }

    public function respond(Request $request, int $invite, string $answer): RedirectResponse
    {
        abort_unless(in_array($answer, ['accept', 'decline'], true), 404);
        $user = $request->user();

        $row = DB::table('chat_group_invites')->where('id', $invite)
            ->where('invitee_id', $user->id)->where('status', 'pending')->first();
        abort_unless($row, 404);

        DB::transaction(function () use ($row, $answer, $user) {
            DB::table('chat_group_invites')->where('id', $row->id)
                ->update(['status' => $answer === 'accept' ? 'accepted' : 'declined', 'updated_at' => now()]);
            if ($answer === 'accept') {
                DB::table('chat_group_members')->insertOrIgnore([
                    'group_id' => $row->group_id, 'user_id' => $user->id, 'created_at' => now(), 'updated_at' => now(),
                ]);
            }
        });

        if ($answer === 'accept' && ($inviter = User::find($row->inviter_id))) {
            SafeMail::to($inviter, new InviteAccepted($user, DB::table('chat_groups')->where('id', $row->group_id)->value('name')));
        }

        return back()->with('status', $answer === 'accept'
            ? 'Joined. The group appears in your terminal once another member is online to share its key.'
            : 'Invitation declined.');
    }

    public function leave(Request $request, int $group): RedirectResponse
    {
        $user = $request->user();
        DB::table('chat_group_members')->where('group_id', $group)->where('user_id', $user->id)->delete();
        DB::table('chat_group_grants')->where('group_id', $group)->where('user_id', $user->id)->delete();
        DB::table('chat_group_invites')->where('group_id', $group)->where('invitee_id', $user->id)->delete();
        // A group nobody belongs to any more goes away.
        if (! DB::table('chat_group_members')->where('group_id', $group)->exists()) {
            DB::table('chat_groups')->where('id', $group)->delete();
        }

        return back()->with('status', 'You left the group.');
    }
}
