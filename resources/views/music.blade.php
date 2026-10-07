@extends('layouts.app')
@section('title', 'Music')
@section('content')
<div class="mb-8">
    <p class="font-mono text-xs uppercase tracking-[0.2em] text-cyber-neon">// music</p>
    <h1 class="font-mono text-3xl font-black text-white">Music</h1>
    <p class="mt-2 text-sm text-gray-400">A shared library. Upload tracks here, play them here or in the terminal with <span class="font-mono text-cyber-cyan">/music</span> and <span class="font-mono text-cyber-cyan">/play</span>.</p>
</div>

<div class="mb-8 flex flex-wrap items-center gap-4">
    <button type="button" data-open-modal="upload-music" class="bg-cyber-neon px-4 py-2.5 font-mono text-xs font-bold uppercase tracking-wider text-black transition-all hover:bg-emerald-400 neon-glow-green"><i class="fa-solid fa-upload mr-2"></i>Upload music</button>
    <span class="font-mono text-xs text-gray-500">{{ $tracks->count() }} {{ \Illuminate\Support\Str::plural('track', $tracks->count()) }} in the library</span>
</div>

{{-- Upload music: a pop-up with a drop box. Files or a whole folder can be dropped or chosen, and are sent one at a time. --}}
<dialog id="upload-music" class="w-full max-w-2xl border border-cyber-600 bg-cyber-800 p-0 text-gray-200 shadow-2xl backdrop:bg-black/70 backdrop:backdrop-blur-sm"
        data-url="{{ route('music.store') }}" data-extensions="{{ implode(',', \App\Http\Controllers\MusicController::EXTENSIONS) }}" data-max-mb="50">
    <div class="flex items-start justify-between gap-4 border-b border-cyber-600 p-5">
        <div>
            <h2 class="font-mono text-lg font-bold text-white">Upload music</h2>
            <p class="mt-1 text-xs text-gray-400">MP3, M4A, AAC, OGG, OPUS, WAV or FLAC, up to 50 MB each. Songs already in the library are skipped.</p>
        </div>
        <button type="button" data-close-upload aria-label="Close" class="h-8 w-8 shrink-0 text-gray-400 hover:bg-cyber-700 hover:text-cyber-neon"><i class="fa-solid fa-xmark"></i></button>
    </div>

    <form method="POST" action="{{ route('music.store') }}" enctype="multipart/form-data" class="p-5" data-upload-form>
        @csrf
        <div data-drop-zone tabindex="0" role="button" aria-label="Drop music files or a folder here, or press Enter to choose files"
             class="flex cursor-pointer flex-col items-center justify-center gap-3 border-2 border-dashed border-cyber-600 bg-cyber-900/60 px-6 py-10 text-center transition-colors hover:border-cyber-neon/60 focus-visible:border-cyber-neon focus-visible:outline-none data-[over=true]:border-cyber-neon data-[over=true]:bg-cyber-neon/10">
            <i class="fa-solid fa-cloud-arrow-up text-4xl text-cyber-cyan"></i>
            <p class="font-mono text-sm font-bold text-white">Drag songs or a whole folder here</p>
            <p class="font-mono text-xs text-gray-400">or</p>
            <div class="flex flex-wrap justify-center gap-3">
                <label class="cursor-pointer border border-cyber-600 px-3 py-2 font-mono text-xs font-bold uppercase tracking-wider text-gray-200 transition-all hover:border-cyber-neon hover:text-cyber-neon">
                    <i class="fa-solid fa-file-audio mr-2"></i>Choose files
                    <input type="file" name="files[]" multiple accept=".mp3,.m4a,.aac,.ogg,.oga,.opus,.wav,.flac,audio/*" class="sr-only" data-pick-files>
                </label>
                <label class="cursor-pointer border border-cyber-600 px-3 py-2 font-mono text-xs font-bold uppercase tracking-wider text-gray-200 transition-all hover:border-cyber-neon hover:text-cyber-neon">
                    <i class="fa-solid fa-folder-open mr-2"></i>Choose a folder
                    <input type="file" webkitdirectory directory multiple class="sr-only" data-pick-folder>
                </label>
            </div>
        </div>

        <p class="mt-3 hidden font-mono text-xs text-gray-400" data-summary></p>
        <ul class="mt-3 hidden max-h-64 divide-y divide-cyber-700 overflow-y-auto border border-cyber-700 bg-cyber-900/60" data-list></ul>

        <div class="mt-5 flex flex-wrap items-center justify-between gap-3">
            <p class="font-mono text-[11px] text-gray-500" data-hint>Nothing is sent until you press Upload.</p>
            <div class="flex gap-3">
                <button type="button" data-close-upload class="border border-cyber-600 px-4 py-2.5 font-mono text-xs font-bold uppercase tracking-wider text-gray-300 hover:border-cyber-neon hover:text-cyber-neon">Close</button>
                <button type="submit" disabled data-start class="bg-cyber-neon px-4 py-2.5 font-mono text-xs font-bold uppercase tracking-wider text-black hover:bg-emerald-400 disabled:cursor-not-allowed disabled:opacity-40"><i class="fa-solid fa-upload mr-2"></i><span data-start-label>Upload</span></button>
            </div>
        </div>
    </form>
</dialog>

<div class="space-y-3">
    @forelse ($tracks as $track)
        <div class="border border-cyber-600 bg-cyber-800/90 p-4">
            <div class="flex flex-wrap items-start justify-between gap-3">
                <div class="min-w-0">
                    <p class="truncate font-mono text-sm font-bold text-white"><span class="mr-2 text-cyber-neon">#{{ $track->id }}</span>{{ $track->title }}</p>
                    <p class="truncate font-mono text-xs text-gray-400">{{ $track->artist ?: 'Unknown artist' }} &middot; {{ strtoupper($track->extension) }} &middot; {{ number_format($track->size / 1048576, 1) }} MB &middot; {{ $track->uploader ? '@'.$track->uploader : 'removed user' }}@if ($track->md5) &middot; <span title="MD5 {{ $track->md5 }}">md5 {{ substr($track->md5, 0, 8) }}</span>@endif</p>
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
