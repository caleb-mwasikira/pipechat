@extends('layouts.app')
@section('title', 'Binaries')
@section('content')
<div class="mb-8">
    <p class="font-mono text-xs uppercase tracking-[0.2em] text-cyber-neon">// admin</p>
    <h1 class="font-mono text-3xl font-black text-white">Binaries</h1>
</div>
<x-status />

<a href="{{ route('admin.binaries.create') }}" class="mb-6 inline-block bg-cyber-neon px-4 py-2.5 font-mono text-xs font-bold uppercase tracking-wider text-black transition-all hover:bg-emerald-400 neon-glow-green"><i class="fa-solid fa-upload mr-2"></i>Upload a binary</a>

<div class="overflow-x-auto border border-cyber-600 bg-cyber-800/90">
    <table class="w-full font-mono text-xs">
        <thead><tr class="text-left text-gray-500"><th class="p-3">Platform</th><th class="p-3">Version</th><th class="p-3">Size</th><th class="p-3">MD5</th><th class="p-3">Uploaded</th><th class="p-3"></th></tr></thead>
        <tbody>
        @forelse ($binaries as $b)
            <tr class="border-t border-cyber-700 text-gray-300">
                <td class="p-3">{{ \App\Http\Controllers\DownloadController::OSES[$b->os] }} {{ $b->arch }}</td>
                <td class="p-3 text-cyber-neon">{{ $b->version }}</td>
                <td class="p-3">{{ number_format($b->size / 1048576, 1) }} MB</td>
                <td class="p-3 break-all text-gray-400">{{ $b->md5 }}</td>
                <td class="p-3 text-gray-400">{{ \Illuminate\Support\Carbon::parse($b->created_at)->diffForHumans() }}</td>
                <td class="p-3 text-right">
                    <form method="POST" action="{{ route('admin.binaries.destroy', $b->id) }}" onsubmit="return confirm('Delete {{ $b->filename }}?')">@csrf @method('DELETE')
                        <button class="text-gray-500 hover:text-red-400">delete</button>
                    </form>
                </td>
            </tr>
        @empty
            <tr><td class="p-4 text-gray-500" colspan="6">No binaries uploaded.</td></tr>
        @endforelse
        </tbody>
    </table>
</div>
@endsection
