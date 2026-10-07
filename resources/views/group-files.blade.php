@extends('layouts.app')
@section('title', $group->name.' · Files')
@section('content')
@php
    $active = 'files';
    $size = fn ($bytes) => $bytes >= 1048576 ? number_format($bytes / 1048576, 1).' MB' : number_format($bytes / 1024, 1).' KB';
@endphp
@include('partials.group-header')

<section class="border border-cyber-600 bg-cyber-800/90">
    <h2 class="border-b border-cyber-600 p-4 font-mono text-xs font-bold uppercase tracking-wider text-white">Shared files <span class="ml-2 text-gray-500">{{ $files->count() }}</span></h2>
    @if ($files->isEmpty())
        <p class="p-4 font-mono text-xs text-gray-500">Nothing shared yet. In the terminal, <span class="text-cyber-cyan">/upload PATH</span> in this group shares a file with everyone in it.</p>
    @else
        <div class="overflow-x-auto">
            <table class="w-full font-mono text-xs">
                <thead><tr class="text-left text-gray-500"><th class="p-3">File</th><th class="p-3">MD5</th><th class="p-3">Size</th><th class="p-3">Shared by</th><th class="p-3">When</th><th class="p-3">Availability</th></tr></thead>
                <tbody>
                @foreach ($files as $file)
                    @php $expired = $file->expires_at && \Illuminate\Support\Carbon::parse($file->expires_at)->lt($now); @endphp
                    <tr class="border-t border-cyber-700 text-gray-300">
                        <td class="p-3 break-all text-white">{{ $file->name }}</td>
                        <td class="p-3 text-gray-500" @if ($file->md5) title="{{ $file->md5 }}" @endif>{{ $file->md5 ? substr($file->md5, 0, 8) : '—' }}</td>
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
@endsection
