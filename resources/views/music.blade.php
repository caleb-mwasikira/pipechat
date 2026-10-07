@extends('layouts.app')
@section('title', 'Music')
@section('content')
<div class="mb-8">
    <p class="font-mono text-xs uppercase tracking-[0.2em] text-cyber-neon">// music</p>
    <h1 class="font-mono text-3xl font-black text-white">Music</h1>
    <p class="mt-2 text-sm text-gray-400">A shared library. Upload tracks here, play them here or in the terminal with <span class="font-mono text-cyber-cyan">/music</span> and <span class="font-mono text-cyber-cyan">/play</span>.</p>
</div>
<x-status />

<form method="POST" action="{{ route('music.store') }}" enctype="multipart/form-data" class="mb-8 grid max-w-3xl gap-4 border border-cyber-600 bg-cyber-800/90 p-5 md:grid-cols-2">
    @csrf
    <div class="md:col-span-2">
        <x-label for="file">Audio file (MP3, M4A, AAC, OGG, OPUS, WAV or FLAC, up to 50 MB)</x-label>
        <input type="file" name="file" id="file" required accept=".mp3,.m4a,.aac,.ogg,.oga,.opus,.wav,.flac,audio/*" class="w-full text-sm text-gray-300 file:mr-3 file:border-0 file:bg-cyber-700 file:px-3 file:py-2.5 file:font-mono file:text-xs file:text-gray-200">
        <x-input-error :messages="$errors->get('file')" />
    </div>
    <div>
        <x-label for="title">Title <span class="normal-case text-gray-500">(optional)</span></x-label>
        <x-input name="title" id="title" value="{{ old('title') }}" maxlength="120" placeholder="from the file name" />
        <x-input-error :messages="$errors->get('title')" />
    </div>
    <div>
        <x-label for="artist">Artist <span class="normal-case text-gray-500">(optional)</span></x-label>
        <x-input name="artist" id="artist" value="{{ old('artist') }}" maxlength="120" />
        <x-input-error :messages="$errors->get('artist')" />
    </div>
    <div class="md:col-span-2"><x-button type="submit"><i class="fa-solid fa-upload mr-2"></i>Upload</x-button></div>
</form>

<div class="space-y-3">
    @forelse ($tracks as $track)
        <div class="border border-cyber-600 bg-cyber-800/90 p-4">
            <div class="flex flex-wrap items-start justify-between gap-3">
                <div class="min-w-0">
                    <p class="truncate font-mono text-sm font-bold text-white"><span class="mr-2 text-cyber-neon">#{{ $track->id }}</span>{{ $track->title }}</p>
                    <p class="truncate font-mono text-xs text-gray-400">{{ $track->artist ?: 'Unknown artist' }} &middot; {{ strtoupper($track->extension) }} &middot; {{ number_format($track->size / 1048576, 1) }} MB &middot; {{ $track->uploader ? '@'.$track->uploader : 'removed user' }}</p>
                </div>
                @if (auth()->user()->is_admin || $track->uploaded_by === auth()->id())
                    <form method="POST" action="{{ route('music.destroy', $track->id) }}" onsubmit="return confirm('Delete {{ addslashes($track->title) }}?')">@csrf @method('DELETE')
                        <button class="font-mono text-[11px] uppercase tracking-wider text-gray-500 hover:text-red-400">delete</button>
                    </form>
                @endif
            </div>
            <audio controls preload="none" class="mt-3 w-full" src="{{ route('music.audio', $track->id) }}"></audio>
        </div>
    @empty
        <p class="font-mono text-sm text-gray-500">No tracks yet. Upload the first one.</p>
    @endforelse
</div>
@endsection
