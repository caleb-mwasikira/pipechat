<?php

namespace App\Http\Controllers;

use App\Mail\InviteAccepted;
use App\Models\User;
use App\Support\SafeMail;
use Illuminate\Http\RedirectResponse;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\DB;

/** The page behind a group's invite link: who invited you, and a button to join. */
class InviteLinkController extends Controller
{
    private const MAX_GROUPS_PER_USER = 50;

    private function group(string $token): object
    {
        // A short, random token: look it up in one query, and say "not found" for anything else.
        return DB::table('chat_groups')->where('invite_token', $token)->first() ?? abort(404);
    }

    public function show(Request $request, string $token)
    {
        $group = $this->group($token);
        if (DB::table('chat_group_members')->where('group_id', $group->id)->where('user_id', $request->user()->id)->exists()) {
            return redirect()->route('groups.show', $group->id)->with('status', "You are already in {$group->name}.");
        }

        return view('invite-link', [
            'group' => $group,
            'owner' => User::find($group->owner_id),
            'members' => DB::table('chat_group_members')->where('group_id', $group->id)->count(),
            'token' => $token,
        ]);
    }

    public function join(Request $request, string $token): RedirectResponse
    {
        $group = $this->group($token);
        $user = $request->user();

        if (DB::table('chat_group_members')->where('user_id', $user->id)->count() >= self::MAX_GROUPS_PER_USER) {
            return back()->with('error', 'You are in too many groups to join another.');
        }
        $added = DB::table('chat_group_members')->insertOrIgnore([
            'group_id' => $group->id, 'user_id' => $user->id, 'created_at' => now(), 'updated_at' => now(),
        ]);
        if ($added && ($owner = User::find($group->owner_id))) {
            SafeMail::to($owner, new InviteAccepted($user, $group->name));
        }

        return redirect()->route('groups.show', $group->id)
            ->with('status', 'Joined. The group appears in your terminal once another member is online to share its key.');
    }
}
