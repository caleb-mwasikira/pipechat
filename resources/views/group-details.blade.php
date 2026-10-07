@extends('layouts.app')
@section('title', $group->name)
@section('content')
@php
    $size = fn ($bytes) => $bytes >= 1048576 ? number_format($bytes / 1048576, 1).' MB' : number_format($bytes / 1024, 1).' KB';
    $stats = [
        'Members' => $members->count(),
        'Online now' => $onlineCount,
        'Messages stored' => number_format($messages['stored']),
        'Last message' => $messages['last'] ? $messages['last']->diffForHumans() : 'none yet',
        'Files shared' => $files->count(),
        'Music tracks' => $tracks->count(),
    ];
@endphp
<div class="mb-8">
    <a href="{{ route('groups') }}" class="font-mono text-xs text-gray-400 hover:text-cyber-neon">&larr; all groups</a>
    <h1 class="mt-2 flex flex-wrap items-center gap-3 font-mono text-3xl font-black text-white">{{ $group->name }}
        <span class="border px-2 py-0.5 text-[11px] font-normal uppercase tracking-wider {{ $group->is_public ? 'border-cyber-cyan/50 text-cyber-cyan' : 'border-cyber-600 text-gray-400' }}"><i class="fa-solid {{ $group->is_public ? 'fa-globe' : 'fa-lock' }} mr-1"></i>{{ $group->is_public ? 'public' : 'private' }}</span>
    </h1>
    <p class="mt-1 font-mono text-xs text-gray-400">
        Made by {{ $owner ? '@'.$owner->handle : 'a removed user' }} &middot; {{ \Illuminate\Support\Carbon::parse($group->created_at)->format('j M Y') }}
        &middot; {{ $group->gid ? 'created in the terminal' : 'waiting for the owner\'s terminal to create it' }}
    </p>
</div>
<x-status />

<div class="grid grid-cols-2 gap-3 sm:gap-4 lg:grid-cols-3 xl:grid-cols-6">
    @foreach ($stats as $label => $value)
        <div class="corners border border-cyber-600 bg-cyber-800/90 p-4">
            <p class="font-mono text-[11px] uppercase tracking-wider text-gray-400">{{ $label }}</p>
            <p class="mt-2 break-words font-mono text-xl font-black {{ $label === 'Online now' && $value > 0 ? 'text-cyber-neon text-glow-green' : 'text-white' }}">{{ $value }}</p>
        </div>
    @endforeach
</div>

<div class="mt-8 grid gap-6 lg:grid-cols-3">
    <section class="border border-cyber-600 bg-cyber-800/90 lg:col-span-1">
        <h2 class="border-b border-cyber-600 p-4 font-mono text-xs font-bold uppercase tracking-wider text-white">Members</h2>
        <ul class="divide-y divide-cyber-700">
            @foreach ($members as $member)
                <li class="p-4">
                    <div class="flex items-center gap-2 font-mono text-sm">
                        <span class="h-2 w-2 shrink-0 rounded-full {{ $member->is_online ? 'bg-cyber-neon shadow-[0_0_6px_#00ff66]' : 'bg-gray-600' }}" title="{{ $member->is_online ? 'online' : 'offline' }}"></span>
                        <span class="truncate font-bold text-white">{{ '@'.$member->handle }}</span>
                        @if ($member->is_owner)<span class="text-[10px] uppercase tracking-wider text-cyber-purple">creator</span>@endif
                    </div>
                    <p class="mt-1 truncate font-mono text-[11px] text-gray-400">
                        {{ $member->name }} &middot;
                        @if ($member->is_online) online now
                        @elseif ($member->last_seen ?? false) seen {{ \Illuminate\Support\Carbon::createFromTimestamp($member->last_seen)->diffForHumans() }}
                        @else not seen yet @endif
                    </p>
                    @if ($member->bio)<p class="mt-1 text-xs text-gray-300">{{ $member->bio }}</p>@endif
                    @unless ($member->delivered)<p class="mt-1 font-mono text-[11px] text-amber-300">waiting for the group key</p>@endunless
                </li>
            @endforeach
        </ul>
        @if ($invited->isNotEmpty())
            <p class="border-t border-cyber-700 p-4 font-mono text-[11px] text-gray-400">Invited, not answered yet: {{ $invited->map(fn ($h) => '@'.$h)->implode(', ') }}</p>
        @endif
        <form method="POST" action="{{ route('groups.invite', $group->id) }}" class="flex gap-2 border-t border-cyber-700 p-4">
            @csrf
            <input name="who" placeholder="invite by handle or email" required class="min-w-0 flex-1 bg-cyber-900 border border-cyber-600 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-cyber-neon">
            <button class="bg-cyber-700 px-3 font-mono text-xs font-bold uppercase tracking-wider text-gray-300 hover:text-cyber-neon">Invite</button>
        </form>
        @if ($errors->has('who'))<p class="px-4 pb-4 text-xs text-cyber-danger">{{ $errors->first('who') }}</p>@endif
    </section>

    <div class="space-y-6 lg:col-span-2">
        <section class="border border-cyber-600 bg-cyber-800/90">
            <h2 class="border-b border-cyber-600 p-4 font-mono text-xs font-bold uppercase tracking-wider text-white">Shared files</h2>
            @if ($files->isEmpty())
                <p class="p-4 font-mono text-xs text-gray-500">Nothing shared yet. In the terminal, <span class="text-cyber-cyan">/upload PATH</span> in this group shares a file with everyone in it.</p>
            @else
                <div class="overflow-x-auto">
                    <table class="w-full font-mono text-xs">
                        <thead><tr class="text-left text-gray-500"><th class="p-3">File</th><th class="p-3">Size</th><th class="p-3">Shared by</th><th class="p-3">When</th><th class="p-3">Availability</th></tr></thead>
                        <tbody>
                        @foreach ($files as $file)
                            @php $expired = $file->expires_at && \Illuminate\Support\Carbon::parse($file->expires_at)->lt($now); @endphp
                            <tr class="border-t border-cyber-700 text-gray-300">
                                <td class="p-3 break-all text-white">{{ $file->name }}</td>
                                <td class="p-3">{{ $size($file->size) }}</td>
                                <td class="p-3">{{ $file->sharer ? '@'.$file->sharer : 'removed user' }}</td>
                                <td class="p-3 text-gray-400">{{ \Illuminate\Support\Carbon::parse($file->created_at)->diffForHumans() }}</td>
                                <td class="p-3 {{ $expired ? 'text-gray-500' : 'text-cyber-neon' }}">{{ $expired ? 'expired' : ($file->expires_at ? 'expires '.\Illuminate\Support\Carbon::parse($file->expires_at)->diffForHumans() : 'available') }}</td>
                            </tr>
                        @endforeach
                        </tbody>
                    </table>
                </div>
                <p class="border-t border-cyber-700 p-3 font-mono text-[11px] text-gray-500">Files are encrypted end to end, so they download in the terminal (<span class="text-cyber-cyan">/files</span>, <span class="text-cyber-cyan">/download</span>), not here.</p>
            @endif
        </section>

        <section class="border border-cyber-600 bg-cyber-800/90">
            <h2 class="border-b border-cyber-600 p-4 font-mono text-xs font-bold uppercase tracking-wider text-white">Music from members</h2>
            @forelse ($tracks as $track)
                <div class="border-t border-cyber-700 p-4 first:border-0">
                    <p class="truncate font-mono text-sm font-bold text-white"><span class="mr-2 text-cyber-neon">#{{ $track->id }}</span>{{ $track->title }}</p>
                    <p class="truncate font-mono text-xs text-gray-400">{{ $track->artist ?: 'Unknown artist' }} &middot; {{ strtoupper($track->extension) }} &middot; {{ $size($track->size) }} &middot; {{ '@'.$track->uploader }}</p>
                    <audio controls preload="none" class="mt-2 w-full" src="{{ route('music.audio', $track->id) }}"></audio>
                </div>
            @empty
                <p class="p-4 font-mono text-xs text-gray-500">None of this group's members has uploaded music yet. <a href="{{ route('music') }}" class="text-cyber-cyan hover:underline">Upload a track</a>.</p>
            @endforelse
        </section>

        @if ($group->owner_id === auth()->id())
            <form method="POST" action="{{ route('groups.visibility', $group->id) }}" class="flex flex-wrap items-center gap-3 border border-cyber-600 bg-cyber-800/90 p-4">
                @csrf
                <input type="hidden" name="public" value="{{ $group->is_public ? 0 : 1 }}">
                <p class="min-w-0 flex-1 font-mono text-xs text-gray-400">{{ $group->is_public ? 'Anyone can join this group from the public list.' : 'People need an invitation to join this group.' }}</p>
                <button class="border border-cyber-600 px-3 py-2 font-mono text-xs font-bold uppercase tracking-wider text-gray-300 hover:border-cyber-neon hover:text-cyber-neon">Make it {{ $group->is_public ? 'private' : 'public' }}</button>
            </form>
            @if ($errors->has('visibility'))<p class="text-xs text-cyber-danger">{{ $errors->first('visibility') }}</p>@endif
        @endif

        <form method="POST" action="{{ route('groups.leave', $group->id) }}" onsubmit="return confirm('Leave {{ addslashes($group->name) }}?')">
            @csrf
            <button class="font-mono text-xs uppercase tracking-wider text-gray-500 hover:text-red-400"><i class="fa-solid fa-right-from-bracket mr-1.5"></i>Leave this group</button>
        </form>
    </div>
</div>
@endsection
