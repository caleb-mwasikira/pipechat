{{-- The top of every page of a group: its name, whether it is public, who made it, and the tabs. --}}
@php
    $tabs = [
        'overview' => ['Overview', route('groups.show', $group->id), null],
        'files' => ['Files', route('groups.files', $group->id), $counts['files']],
        'music' => ['Music', route('groups.music', $group->id), $counts['tracks']],
    ];
    $isOwner = $group->owner_id === auth()->id();
@endphp
<div class="mb-6">
    <a href="{{ route('groups') }}" class="font-mono text-xs text-gray-400 hover:text-cyber-neon">&larr; all groups</a>
    <h1 class="mt-2 flex flex-wrap items-center gap-3 font-mono text-3xl font-black text-white">
        <span class="break-all">{{ $group->name }}</span>
        <span class="border px-2 py-0.5 text-[11px] font-normal uppercase tracking-wider {{ $group->is_public ? 'border-cyber-cyan/50 text-cyber-cyan' : 'border-cyber-600 text-gray-400' }}"><i class="fa-solid {{ $group->is_public ? 'fa-globe' : 'fa-lock' }} mr-1"></i>{{ $group->is_public ? 'public' : 'private' }}</span>
        @if ($isOwner)
            <button type="button" data-open-dialog="rename-group" title="Rename this group" class="border border-cyber-600 px-2 py-1 font-mono text-[11px] font-normal uppercase tracking-wider text-gray-300 hover:border-cyber-neon hover:text-cyber-neon"><i class="fa-solid fa-pen mr-1"></i>Rename</button>
        @endif
    </h1>
    <p class="mt-1 font-mono text-xs text-gray-400">
        Made by {{ $owner ? '@'.$owner->handle : 'a removed user' }} &middot; {{ \Illuminate\Support\Carbon::parse($group->created_at)->format('j M Y') }}
        &middot; <span title="The name can change; this id never does">ID {{ $group->id }}</span>
        &middot; {{ $group->gid ? 'created in the terminal' : 'waiting for the owner\'s terminal to create it' }}
    </p>
</div>

<nav class="mb-8 flex gap-1 border-b border-cyber-700" aria-label="Group pages">
    @foreach ($tabs as $key => [$label, $url, $count])
        <a href="{{ $url }}" @if ($active === $key) aria-current="page" @endif
           class="-mb-px border-b-2 px-4 py-2.5 font-mono text-xs font-bold uppercase tracking-wider transition-colors {{ $active === $key ? 'border-cyber-neon text-cyber-neon' : 'border-transparent text-gray-400 hover:text-white' }}">
            {{ $label }}@if ($count !== null)<span class="ml-1.5 text-[10px] {{ $active === $key ? 'text-cyber-neon/70' : 'text-gray-600' }}">{{ $count }}</span>@endif
        </a>
    @endforeach
</nav>

@if ($isOwner)
    <dialog id="rename-group" class="w-full max-w-md border border-cyber-600 bg-cyber-800 p-0 text-gray-200 shadow-2xl backdrop:bg-black/70 backdrop:backdrop-blur-sm">
        <form method="POST" action="{{ route('groups.rename', $group->id) }}" class="space-y-5 p-6">
            @csrf
            @method('PUT')
            <div class="flex items-start justify-between gap-4">
                <div>
                    <h2 class="font-mono text-lg font-bold text-white">Rename group</h2>
                    <p class="mt-1 text-xs text-gray-400">Members see the new name on the website at once, and in their terminals after the next sync.</p>
                </div>
                <button type="button" data-close-dialog aria-label="Close" class="h-8 w-8 shrink-0 text-gray-400 hover:bg-cyber-700 hover:text-cyber-neon"><i class="fa-solid fa-xmark"></i></button>
            </div>
            <div>
                <x-label for="rename-name">Name</x-label>
                <x-input icon="fa-solid fa-users" name="name" id="rename-name" value="{{ old('name', $group->name) }}" maxlength="24" required />
                <x-input-error :messages="$errors->get('name')" />
                <x-input-error :messages="$errors->get('rename')" />
            </div>
            <div class="flex justify-end gap-3">
                <button type="button" data-close-dialog class="border border-cyber-600 px-4 py-2.5 font-mono text-xs font-bold uppercase tracking-wider text-gray-300 hover:border-cyber-neon hover:text-cyber-neon">Cancel</button>
                <button type="submit" class="bg-cyber-neon px-4 py-2.5 font-mono text-xs font-bold uppercase tracking-wider text-black hover:bg-emerald-400"><i class="fa-solid fa-check mr-2"></i>Save</button>
            </div>
        </form>
    </dialog>
    @if ($errors->has('name') || $errors->has('rename'))
        <script>document.getElementById('rename-group').showModal();</script>
    @endif
@endif
