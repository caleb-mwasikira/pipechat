@extends('layouts.app')
@section('title', 'Groups')
@section('content')
<div class="mb-8">
    <p class="font-mono text-xs uppercase tracking-[0.2em] text-cyber-neon">// groups</p>
    <h1 class="font-mono text-3xl font-black text-white">Your groups</h1>
    <p class="mt-2 text-sm text-gray-400">Groups here show up in your terminal chat. Their encryption key never touches the website: another member's terminal passes it to you.</p>
</div>
<x-status />

@if ($invites->isNotEmpty())
    <section class="mb-8 border border-cyber-neon/40 bg-cyber-800/90 p-5">
        <h2 class="mb-3 font-mono text-xs font-bold uppercase tracking-wider text-cyber-neon">Invitations</h2>
        @foreach ($invites as $invite)
            <div class="flex flex-wrap items-center justify-between gap-3 border-t border-cyber-700 py-3 font-mono text-sm first:border-0">
                <span class="text-gray-300"><span class="text-white">{{ $invite->name }}</span> &middot; from @{{ $invite->inviter }}</span>
                <span class="flex gap-2">
                    @foreach (['accept' => 'bg-cyber-neon text-black hover:bg-emerald-400', 'decline' => 'border border-cyber-600 text-gray-300 hover:border-red-400 hover:text-red-400'] as $answer => $class)
                        <form method="POST" action="{{ route('invites.respond', [$invite->id, $answer]) }}">@csrf
                            <button class="px-3 py-1.5 text-xs font-bold uppercase tracking-wider transition-all {{ $class }}">{{ $answer }}</button>
                        </form>
                    @endforeach
                </span>
            </div>
        @endforeach
    </section>
@endif

<section class="mb-8 max-w-md border border-cyber-600 bg-cyber-800/90 p-5">
    <form method="POST" action="{{ route('groups.store') }}" class="space-y-3">
        @csrf
        <x-label for="name">Create a group</x-label>
        <x-input icon="fa-solid fa-users" name="name" id="name" value="{{ old('name') }}" placeholder="group name (max 24)" maxlength="24" required />
        <x-input-error :messages="$errors->get('name')" />
        <x-button type="submit"><i class="fa-solid fa-plus mr-2"></i>Create</x-button>
    </form>
</section>

<div class="grid gap-6 md:grid-cols-2">
    @forelse ($groups as $group)
        <div class="corners border border-cyber-600 bg-cyber-800/90 p-5">
            <div class="mb-1 flex items-center justify-between">
                <h2 class="font-mono text-lg font-bold text-white">{{ $group->name }}</h2>
                <form method="POST" action="{{ route('groups.leave', $group->id) }}" onsubmit="return confirm('Leave this group?')">@csrf
                    <button class="font-mono text-[11px] uppercase tracking-wider text-gray-500 hover:text-red-400">leave</button>
                </form>
            </div>
            <p class="mb-3 font-mono text-[11px] {{ $group->delivered ? 'text-cyber-neon' : 'text-amber-300' }}">
                {{ $group->gid ? ($group->delivered ? 'in your terminal' : 'waiting for a member to share the key') : 'waiting for the owner\'s terminal to create it' }}
            </p>
            <p class="mb-4 font-mono text-xs text-gray-400">{{ $group->members->map(fn ($h) => '@'.$h)->implode('  ') }}</p>
            <form method="POST" action="{{ route('groups.invite', $group->id) }}" class="flex gap-2">
                @csrf
                <input name="who" placeholder="handle or email" required class="min-w-0 flex-1 bg-cyber-900 border border-cyber-600 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-cyber-neon">
                <button class="bg-cyber-700 px-3 font-mono text-xs font-bold uppercase tracking-wider text-gray-300 hover:text-cyber-neon">Invite</button>
            </form>
        </div>
    @empty
        <p class="font-mono text-sm text-gray-500">No groups yet.</p>
    @endforelse
</div>
@if ($errors->has('who'))<p class="mt-4 text-xs text-cyber-danger">{{ $errors->first('who') }}</p>@endif
@endsection
