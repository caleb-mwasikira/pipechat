@extends('layouts.app')
@section('title', $group->name.' · Music')
@section('content')
@php
    $active = 'music';
    $size = fn ($bytes) => $bytes >= 1048576 ? number_format($bytes / 1048576, 1).' MB' : number_format($bytes / 1024, 1).' KB';
@endphp
@include('partials.group-header')

<section class="border border-cyber-600 bg-cyber-800/90">
    <h2 class="border-b border-cyber-600 p-4 font-mono text-xs font-bold uppercase tracking-wider text-white">Music from members <span class="ml-2 text-gray-500">{{ $tracks->count() }}</span></h2>
    @forelse ($tracks as $track)
        <div class="border-t border-cyber-700 p-4 first:border-0">
            <p class="truncate font-mono text-sm font-bold text-white"><span class="mr-2 text-cyber-neon">#{{ $track->id }}</span>{{ $track->title }}</p>
            <p class="truncate font-mono text-xs text-gray-400">{{ $track->artist ?: 'Unknown artist' }} &middot; {{ strtoupper($track->extension) }} &middot; {{ $size($track->size) }} &middot; {{ '@'.$track->uploader }}@if ($track->md5) &middot; <span title="MD5 {{ $track->md5 }}">md5 {{ substr($track->md5, 0, 8) }}</span>@endif</p>
            <audio controls preload="none" class="mt-2 w-full" src="{{ route('music.audio', $track->id) }}"></audio>
        </div>
    @empty
        <p class="p-4 font-mono text-xs text-gray-500">None of this group's members has uploaded music yet. <a href="{{ route('music') }}" class="text-cyber-cyan hover:underline">Upload a track</a>.</p>
    @endforelse
</section>
@endsection
