@extends('layouts.app')
@section('title', 'Download')
@section('content')
@php
    $oses = \App\Http\Controllers\DownloadController::OSES;
    $icons = ['linux' => 'fa-brands fa-linux', 'windows' => 'fa-brands fa-windows', 'macos' => 'fa-brands fa-apple'];
    $run = fn ($b) => $b->os === 'windows' ? ['.\\'.$b->filename] : ['chmod +x '.$b->filename, './'.$b->filename];
@endphp
<div class="mb-8">
    <p class="font-mono text-xs uppercase tracking-[0.2em] text-cyber-neon">// download</p>
    <h1 class="font-mono text-3xl font-black text-white">Get the terminal client</h1>
    <p class="mt-2 text-sm text-gray-400">Single static binary, no installer. Check the MD5 after downloading to make sure it is intact.</p>
</div>

<div class="grid gap-6 md:grid-cols-2">
    @forelse ($builds as $key => $versions)
        @php $latest = $versions->first(); @endphp
        <div class="corners border border-cyber-600 bg-cyber-800/90 p-6">
            <div class="mb-4 flex items-center justify-between gap-3">
                <h2 class="flex items-center gap-3 font-mono text-lg font-bold text-white"><i class="{{ $icons[$latest->os] }} text-2xl text-cyber-cyan"></i>{{ $oses[$latest->os] }} <span class="text-sm text-gray-400">{{ $latest->arch }}</span></h2>
                <span class="border border-cyber-neon/40 px-2 py-0.5 font-mono text-xs text-cyber-neon">v{{ $latest->version }}</span>
            </div>
            <p class="mb-1 font-mono text-xs text-gray-400">{{ $latest->filename }} &middot; {{ number_format($latest->size / 1048576, 1) }} MB</p>
            <p class="mb-4 break-all font-mono text-[11px] text-gray-500">MD5 <span class="text-gray-300">{{ $latest->md5 }}</span></p>
            <a href="{{ route('download.file', $latest->id) }}" class="block bg-cyber-neon py-3 text-center font-mono text-xs font-bold uppercase tracking-wider text-black transition-all hover:bg-emerald-400 neon-glow-green"><i class="fa-solid fa-download mr-2"></i>Download v{{ $latest->version }}</a>
            <div class="mt-4 bg-black border border-cyber-700 p-3 font-mono text-xs text-cyber-cyan">
                <div class="text-gray-500"># verify</div>
                <div><span class="text-cyber-neon">$</span> {{ $latest->os === 'windows' ? 'certutil -hashfile '.$latest->filename.' MD5' : ($latest->os === 'macos' ? 'md5 '.$latest->filename : 'md5sum '.$latest->filename) }}</div>
                @foreach ($run($latest) as $line)<div><span class="text-cyber-neon">$</span> {{ $line }}</div>@endforeach
            </div>
            @if ($versions->count() > 1)
                <details class="mt-4 font-mono text-xs text-gray-400">
                    <summary class="cursor-pointer hover:text-cyber-neon">Older versions</summary>
                    @foreach ($versions->skip(1) as $old)
                        <div class="mt-2 border-t border-cyber-700 pt-2">
                            <a href="{{ route('download.file', $old->id) }}" class="text-gray-200 hover:text-cyber-neon">v{{ $old->version }}</a>
                            <span class="text-gray-500">&middot; {{ number_format($old->size / 1048576, 1) }} MB</span>
                            <div class="break-all text-[11px] text-gray-500">MD5 {{ $old->md5 }}</div>
                        </div>
                    @endforeach
                </details>
            @endif
        </div>
    @empty
        <p class="font-mono text-sm text-gray-500">No downloads are available yet.</p>
    @endforelse
</div>
@endsection
