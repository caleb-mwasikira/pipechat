@extends('layouts.app')
@section('title', $group->name)
@section('content')
@php
    $active = 'overview';
    $stats = [
        ['Members', $members->count(), null],
        ['Online now', $onlineCount, null],
        ['Files shared', $counts['files'], route('groups.files', $group->id)],
        ['Music tracks', $counts['tracks'], route('groups.music', $group->id)],
    ];
@endphp
@include('partials.group-header')

<div class="grid grid-cols-2 gap-3 sm:gap-4 lg:grid-cols-4">
    @foreach ($stats as [$label, $value, $link])
        @php $card = 'corners block border border-cyber-600 bg-cyber-800/90 p-4'; @endphp
        @if ($link)
            <a href="{{ $link }}" class="{{ $card }} transition-colors hover:border-cyber-neon/60">
                <p class="font-mono text-[11px] uppercase tracking-wider text-gray-400">{{ $label }}<i class="fa-solid fa-arrow-right ml-1.5 text-[9px] text-gray-600"></i></p>
                <p class="mt-2 break-words font-mono text-2xl font-black text-white">{{ $value }}</p>
            </a>
        @else
            <div class="{{ $card }}">
                <p class="font-mono text-[11px] uppercase tracking-wider text-gray-400">{{ $label }}</p>
                <p class="mt-2 break-words font-mono text-2xl font-black {{ $label === 'Online now' && $value > 0 ? 'text-cyber-neon text-glow-green' : 'text-white' }}">{{ $value }}</p>
            </div>
        @endif
    @endforeach
</div>

<div class="mt-8 grid gap-6 lg:grid-cols-3">
    <section class="border border-cyber-600 bg-cyber-800/90 lg:col-span-2">
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
    </section>

    <div class="space-y-6">
        <form method="POST" action="{{ route('groups.invite', $group->id) }}" class="space-y-3 border border-cyber-600 bg-cyber-800/90 p-4">
            @csrf
            <x-label for="invite-who">Invite someone</x-label>
            <div class="flex gap-2">
                <input id="invite-who" name="who" placeholder="handle or email" required class="min-w-0 flex-1 bg-cyber-900 border border-cyber-600 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-cyber-neon">
                <button class="bg-cyber-700 px-3 font-mono text-xs font-bold uppercase tracking-wider text-gray-300 hover:text-cyber-neon">Invite</button>
            </div>
            @if ($errors->has('who'))<p class="text-xs text-cyber-danger">{{ $errors->first('who') }}</p>@endif
        </form>

        @if ($group->owner_id === auth()->id())
            {{-- Private or public, as a switch: flipping it saves at once (a button stands in when scripts are off). --}}
            <form method="POST" action="{{ route('groups.visibility', $group->id) }}" data-autosubmit class="border border-cyber-600 bg-cyber-800/90 p-4">
                @csrf
                <input type="hidden" name="public" value="0">
                <label class="flex cursor-pointer items-start gap-4">
                    <span class="relative mt-0.5 inline-block h-6 w-11 shrink-0">
                        <input type="checkbox" name="public" value="1" class="peer sr-only" @checked($group->is_public)>
                        <span class="absolute inset-0 rounded-full bg-cyber-600 transition-colors peer-checked:bg-cyber-neon peer-focus-visible:ring-2 peer-focus-visible:ring-cyber-cyan"></span>
                        <span class="absolute left-0.5 top-0.5 h-5 w-5 rounded-full bg-white transition-transform peer-checked:translate-x-5"></span>
                    </span>
                    <span class="min-w-0">
                        <span class="block font-mono text-sm font-bold text-white">{{ $group->is_public ? 'Public' : 'Private' }}</span>
                        <span class="mt-1 block text-xs leading-relaxed text-gray-400">{{ $group->is_public ? 'Anyone with an account can join from the public groups list.' : 'People need an invitation, or the invite link below, to join.' }}</span>
                    </span>
                </label>
                <noscript><button class="mt-3 border border-cyber-600 px-3 py-2 font-mono text-xs font-bold uppercase tracking-wider text-gray-300">Save</button></noscript>
            </form>
            @if ($errors->has('visibility'))<p class="text-xs text-cyber-danger">{{ $errors->first('visibility') }}</p>@endif

            {{-- The invite link: send it to anyone; opening it (and logging in) offers them a place in the group. --}}
            <div class="space-y-3 border border-cyber-600 bg-cyber-800/90 p-4">
                <p class="font-mono text-xs font-bold uppercase tracking-wider text-white"><i class="fa-solid fa-link mr-2 text-cyber-cyan"></i>Invite link</p>
                @if ($group->invite_token)
                    @php $inviteUrl = route('invite.show', $group->invite_token); @endphp
                    <div class="flex gap-2">
                        <input type="text" readonly value="{{ $inviteUrl }}" id="invite-url" onfocus="this.select()" aria-label="Invite link"
                               class="min-w-0 flex-1 bg-cyber-900 border border-cyber-600 rounded-lg px-3 py-2 font-mono text-xs text-cyber-cyan focus:outline-none focus:border-cyber-neon">
                        <button type="button" data-copy="{{ $inviteUrl }}" class="shrink-0 bg-cyber-neon px-3 font-mono text-xs font-bold uppercase tracking-wider text-black hover:bg-emerald-400"><i class="fa-regular fa-copy mr-1.5"></i>Copy</button>
                    </div>
                    <p class="font-mono text-[11px] leading-relaxed text-gray-500">Anyone who opens this link while logged in can join, so share it only with people you want in.</p>
                    <div class="flex flex-wrap gap-4 font-mono text-[11px] uppercase tracking-wider">
                        @foreach (['reset' => ['Make a new link', 'Replace the link? The old one stops working.'], 'disable' => ['Turn off', 'Turn the link off?']] as $action => [$label, $ask])
                            <form method="POST" action="{{ route('groups.invite-link', $group->id) }}" onsubmit="return confirm('{{ $ask }}')">
                                @csrf <input type="hidden" name="action" value="{{ $action }}">
                                <button class="text-gray-500 {{ $action === 'disable' ? 'hover:text-red-400' : 'hover:text-cyber-neon' }}">{{ $label }}</button>
                            </form>
                        @endforeach
                    </div>
                @else
                    <p class="font-mono text-xs text-gray-400">The link is turned off, so nobody can join with it.</p>
                    <form method="POST" action="{{ route('groups.invite-link', $group->id) }}">
                        @csrf <input type="hidden" name="action" value="enable">
                        <button class="bg-cyber-neon px-3 py-2 font-mono text-xs font-bold uppercase tracking-wider text-black hover:bg-emerald-400">Turn on the invite link</button>
                    </form>
                @endif
            </div>

            {{-- Invite codes: short, for people who are already in the terminal. They type /join CODE. --}}
            <div class="space-y-3 border border-cyber-600 bg-cyber-800/90 p-4">
                <p class="font-mono text-xs font-bold uppercase tracking-wider text-white"><i class="fa-solid fa-key mr-2 text-cyber-cyan"></i>Invite codes</p>
                <p class="font-mono text-[11px] leading-relaxed text-gray-500">Give someone a code and they type <span class="text-cyber-cyan">/join CODE</span> in the terminal client. A code works for 7 days and 25 people.</p>
                <form method="POST" action="{{ route('groups.codes.create', $group->id) }}">
                    @csrf
                    <button class="bg-cyber-neon px-3 py-2 font-mono text-xs font-bold uppercase tracking-wider text-black hover:bg-emerald-400"><i class="fa-solid fa-wand-magic-sparkles mr-2"></i>Generate a code</button>
                </form>
                @foreach ($codes as $code)
                    @php $shown = substr($code->code, 0, 4).'-'.substr($code->code, 4); $fresh = session('new_code') === $code->code; @endphp
                    <div class="border {{ $fresh ? 'border-cyber-neon' : 'border-cyber-700' }} bg-cyber-900/60 p-3">
                        <div class="flex items-center justify-between gap-3">
                            <span class="font-mono text-xl font-black tracking-widest text-cyber-neon">{{ $shown }}</span>
                            <span class="flex shrink-0 items-center gap-3">
                                <button type="button" data-copy="{{ '/join '.$shown }}" class="font-mono text-[11px] uppercase tracking-wider text-gray-300 hover:text-cyber-neon"><i class="fa-regular fa-copy mr-1"></i>Copy</button>
                                <form method="POST" action="{{ route('groups.codes.revoke', [$group->id, $code->id]) }}" onsubmit="return confirm('Revoke this code?')">@csrf @method('DELETE')
                                    <button class="font-mono text-[11px] uppercase tracking-wider text-gray-500 hover:text-red-400">Revoke</button>
                                </form>
                            </span>
                        </div>
                        <p class="mt-1 font-mono text-[11px] text-gray-500">used {{ $code->uses }} of {{ $code->max_uses }} &middot; expires {{ \Illuminate\Support\Carbon::parse($code->expires_at)->diffForHumans() }}</p>
                    </div>
                @endforeach
            </div>
        @endif

        <form method="POST" action="{{ route('groups.leave', $group->id) }}" onsubmit="return confirm('Leave {{ addslashes($group->name) }}?')">
            @csrf
            <button class="font-mono text-xs uppercase tracking-wider text-gray-500 hover:text-red-400"><i class="fa-solid fa-right-from-bracket mr-1.5"></i>Leave this group</button>
        </form>
    </div>
</div>
@endsection
