@extends('layouts.app')
@section('title', 'Groups')
@section('content')
<div class="mb-8 flex flex-wrap items-end justify-between gap-4">
    <div>
        <p class="font-mono text-xs uppercase tracking-[0.2em] text-cyber-neon">// groups</p>
        <h1 class="font-mono text-3xl font-black text-white">Your groups</h1>
        <p class="mt-2 max-w-xl text-sm text-gray-400">Groups here show up in your terminal chat. Their encryption key never touches the website: another member's terminal passes it to you.</p>
    </div>
    <button type="button" data-open-modal="create-group" class="bg-cyber-neon px-4 py-2.5 font-mono text-xs font-bold uppercase tracking-wider text-black transition-all hover:bg-emerald-400 neon-glow-green"><i class="fa-solid fa-plus mr-2"></i>Create group</button>
</div>
<x-status />
@if ($errors->has('join'))<p class="mb-4 text-xs text-cyber-danger">{{ $errors->first('join') }}</p>@endif

@if ($invites->isNotEmpty())
    <section class="mb-8 border border-cyber-neon/40 bg-cyber-800/90 p-5">
        <h2 class="mb-3 font-mono text-xs font-bold uppercase tracking-wider text-cyber-neon">Invitations</h2>
        @foreach ($invites as $invite)
            <div class="flex flex-wrap items-center justify-between gap-3 border-t border-cyber-700 py-3 font-mono text-sm first:border-0">
                <span class="text-gray-300"><span class="text-white">{{ $invite->name }}</span> &middot; from {{ '@'.$invite->inviter }}</span>
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

<div class="grid gap-6 md:grid-cols-2">
    @forelse ($groups as $group)
        <div class="corners border border-cyber-600 bg-cyber-800/90 p-5">
            <div class="mb-1 flex items-center justify-between gap-3">
                <h2 class="min-w-0 truncate font-mono text-lg font-bold"><a href="{{ route('groups.show', $group->id) }}" class="text-white hover:text-cyber-neon">{{ $group->name }} <i class="fa-solid fa-arrow-right ml-1 text-xs text-gray-500"></i></a></h2>
                <span class="shrink-0 border px-2 py-0.5 font-mono text-[10px] uppercase tracking-wider {{ $group->is_public ? 'border-cyber-cyan/50 text-cyber-cyan' : 'border-cyber-600 text-gray-400' }}"><i class="fa-solid {{ $group->is_public ? 'fa-globe' : 'fa-lock' }} mr-1"></i>{{ $group->is_public ? 'public' : 'private' }}</span>
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
        <p class="font-mono text-sm text-gray-500">No groups yet. Create one, or join a public group below.</p>
    @endforelse
</div>
@if ($errors->has('who'))<p class="mt-4 text-xs text-cyber-danger">{{ $errors->first('who') }}</p>@endif

<section class="mt-10">
    <h2 class="mb-4 font-mono text-xs font-bold uppercase tracking-wider text-white"><i class="fa-solid fa-globe mr-2 text-cyber-cyan"></i>Public groups you can join</h2>
    <div class="grid gap-4 md:grid-cols-2">
        @forelse ($publicGroups as $public)
            <div class="flex items-center justify-between gap-3 border border-cyber-600 bg-cyber-800/90 p-4">
                <div class="min-w-0">
                    <p class="truncate font-mono text-sm font-bold text-white">{{ $public->name }}</p>
                    <p class="truncate font-mono text-[11px] text-gray-400">by {{ '@'.$public->owner }} &middot; {{ $public->members }} {{ \Illuminate\Support\Str::plural('member', $public->members) }}</p>
                </div>
                <form method="POST" action="{{ route('groups.join', $public->id) }}">@csrf
                    <button class="bg-cyber-neon px-3 py-1.5 font-mono text-xs font-bold uppercase tracking-wider text-black hover:bg-emerald-400">Join</button>
                </form>
            </div>
        @empty
            <p class="font-mono text-xs text-gray-500">No public groups to join right now.</p>
        @endforelse
    </div>
</section>

{{-- Create group: a pop-up. The toggle picks between private (invitation needed) and public (anyone can join). --}}
<dialog id="create-group" class="w-full max-w-md border border-cyber-600 bg-cyber-800 p-0 text-gray-200 shadow-2xl backdrop:bg-black/70 backdrop:backdrop-blur-sm">
    <form method="POST" action="{{ route('groups.store') }}" class="space-y-5 p-6">
        @csrf
        <div class="flex items-start justify-between gap-4">
            <div>
                <h2 class="font-mono text-lg font-bold text-white">Create a group</h2>
                <p class="mt-1 text-xs text-gray-400">It appears in your terminal after it syncs.</p>
            </div>
            <button type="button" data-close-modal aria-label="Close" class="h-8 w-8 text-gray-400 hover:bg-cyber-700 hover:text-cyber-neon"><i class="fa-solid fa-xmark"></i></button>
        </div>

        <div>
            <x-label for="group-name">Name</x-label>
            <x-input icon="fa-solid fa-users" name="name" id="group-name" value="{{ old('name') }}" placeholder="up to 24 characters" maxlength="24" required />
            <x-input-error :messages="$errors->get('name')" />
        </div>

        <label class="flex cursor-pointer items-start gap-4 border border-cyber-600 bg-cyber-900/60 p-4">
            <span class="relative mt-0.5 inline-block h-6 w-11 shrink-0">
                <input type="checkbox" name="public" value="1" id="group-public" class="peer sr-only" @checked(old('public'))>
                <span class="absolute inset-0 rounded-full bg-cyber-600 transition-colors peer-checked:bg-cyber-neon peer-focus-visible:ring-2 peer-focus-visible:ring-cyber-cyan"></span>
                <span class="absolute left-0.5 top-0.5 h-5 w-5 rounded-full bg-white transition-transform peer-checked:translate-x-5"></span>
            </span>
            <span class="min-w-0">
                <span class="block font-mono text-sm font-bold text-white" data-visibility-title>Private</span>
                <span class="mt-1 block text-xs leading-relaxed text-gray-400" data-visibility-text>People need an invitation from a member to join.</span>
            </span>
        </label>

        <div class="flex justify-end gap-3">
            <button type="button" data-close-modal class="border border-cyber-600 px-4 py-2.5 font-mono text-xs font-bold uppercase tracking-wider text-gray-300 hover:border-cyber-neon hover:text-cyber-neon">Cancel</button>
            <button type="submit" class="bg-cyber-neon px-4 py-2.5 font-mono text-xs font-bold uppercase tracking-wider text-black hover:bg-emerald-400"><i class="fa-solid fa-plus mr-2"></i>Create</button>
        </div>
    </form>
</dialog>

<script>
(() => {
    const modal = document.getElementById('create-group');
    const toggle = document.getElementById('group-public');
    const title = modal.querySelector('[data-visibility-title]');
    const text = modal.querySelector('[data-visibility-text]');
    const describe = () => {
        title.textContent = toggle.checked ? 'Public' : 'Private';
        text.textContent = toggle.checked
            ? 'Anyone with an account can join from the public groups list. No invitation needed.'
            : 'People need an invitation from a member to join.';
    };
    toggle.addEventListener('change', describe);
    describe();

    document.querySelectorAll('[data-open-modal="create-group"]').forEach((button) => button.addEventListener('click', () => modal.showModal()));
    modal.querySelectorAll('[data-close-modal]').forEach((button) => button.addEventListener('click', () => modal.close()));
    // A click on the dimmed area outside the box closes it too.
    modal.addEventListener('click', (event) => { if (event.target === modal) modal.close(); });
    @if ($errors->has('name') || $errors->has('public'))
        modal.showModal(); // the form had a mistake: show it again with the message
    @endif
})();
</script>
@endsection
