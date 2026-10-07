<?php

namespace App\Http\Controllers;

use Illuminate\Http\RedirectResponse;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\DB;

class ProfileController extends Controller
{
    public function edit(Request $request)
    {
        return view('profile', ['user' => $request->user()]);
    }

    public function update(Request $request): RedirectResponse
    {
        $data = $request->validate([
            'name' => ['required', 'string', 'max:255'],
            'bio' => ['nullable', 'string', 'max:280'],
        ]);

        // One line of plain text: the terminal shows it on a page of its own.
        $bio = trim(preg_replace('/[\x00-\x1f\x7f\s]+/u', ' ', $data['bio'] ?? ''));

        DB::table('users')->where('id', $request->user()->id)
            ->update(['name' => $data['name'], 'bio' => $bio === '' ? null : $bio, 'updated_at' => now()]);

        return back()->with('status', 'Profile saved. People see it with /whois @'.$request->user()->handle.'.');
    }
}
