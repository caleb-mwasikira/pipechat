<?php

namespace App\Http\Controllers;

use App\Models\User;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Facades\Schema;

class AdminController extends Controller
{
    public function index()
    {
        // The chat server's tables are created by the server itself, so they may not exist yet.
        $count = fn (string $table) => Schema::hasTable($table) ? DB::table($table)->count() : 0;
        $counter = fn (string $name) => Schema::hasTable('counters')
            ? (int) DB::table('counters')->where('name', $name)->value('value')
            : 0;

        $days = 30;
        $perDay = User::query()
            ->where('created_at', '>=', now()->subDays($days - 1)->startOfDay())
            ->get(['created_at'])
            ->countBy(fn ($user) => $user->created_at->toDateString());
        $signups = collect(range($days - 1, 0))->map(function ($ago) use ($perDay) {
            $date = now()->subDays($ago)->toDateString();

            return ['date' => $date, 'count' => $perDay[$date] ?? 0];
        });

        return view('admin', [
            'stats' => [
                'Users registered' => User::count(),
                'Joined, last 7 days' => User::where('created_at', '>=', now()->subDays(7))->count(),
                'Joined, last 30 days' => User::where('created_at', '>=', now()->subDays(30))->count(),
                'Chat accounts' => $count('accounts'),
                'Groups' => $count('groups'),
                'Messages sent (all time)' => $counter('messages_sent'),
                'Messages stored now' => $count('messages'),
                'Files shared (all time)' => $counter('files_sent'),
                'Music tracks' => $count('tracks'),
                'Admins' => User::where('is_admin', true)->count(),
            ],
            'signups' => $signups,
            'recent' => User::latest()->limit(10)->get(['id', 'name', 'email', 'is_admin', 'created_at']),
        ]);
    }
}
