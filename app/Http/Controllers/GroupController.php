<?php

namespace App\Http\Controllers;

use App\Mail\GroupInvite;
use App\Mail\InviteAccepted;
use App\Models\User;
use App\Support\SafeMail;
use Illuminate\Http\RedirectResponse;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\DB;

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
                ->select('g.id', 'g.name', 'g.owner_id', 'g.gid', 'm.delivered')
                ->orderBy('g.name')->get()
                ->each(fn ($g) => $g->members = DB::table('chat_group_members as m')
                    ->join('users as u', 'u.id', '=', 'm.user_id')
                    ->where('m.group_id', $g->id)->orderBy('u.handle')->pluck('u.handle')),
            'invites' => DB::table('chat_group_invites as i')
                ->join('chat_groups as g', 'g.id', '=', 'i.group_id')
                ->join('users as u', 'u.id', '=', 'i.inviter_id')
                ->where('i.invitee_id', $id)->where('i.status', 'pending')
                ->select('i.id', 'g.name', 'u.handle as inviter')->get(),
        ]);
    }

    public function store(Request $request): RedirectResponse
    {
        $data = $request->validate(['name' => ['required', 'string', 'max:24', 'regex:/^[^\x00-\x1f\x7f]+$/u']]);
        $user = $request->user();

        if (DB::table('chat_group_members')->where('user_id', $user->id)->count() >= self::MAX_GROUPS_PER_USER) {
            return back()->withErrors(['name' => 'You are in too many groups.']);
        }

        DB::transaction(function () use ($data, $user) {
            $group = DB::table('chat_groups')->insertGetId([
                'name' => $data['name'], 'owner_id' => $user->id, 'created_at' => now(), 'updated_at' => now(),
            ]);
            DB::table('chat_group_members')->insert([
                'group_id' => $group, 'user_id' => $user->id, 'created_at' => now(), 'updated_at' => now(),
            ]);
        });

        return back()->with('status', 'Group created. It appears in your terminal the next time it syncs.');
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
