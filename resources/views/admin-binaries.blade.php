@extends('layouts.app')
@section('title', 'Binaries')
@section('container', 'max-w-6xl')
@section('content')
<div class="mb-8">
    <p class="font-mono text-xs uppercase tracking-[0.2em] text-cyber-neon">// admin</p>
    <h1 class="font-mono text-3xl font-black text-white">Binaries</h1>
    <a href="{{ route('admin') }}" class="font-mono text-xs text-gray-400 hover:text-cyber-neon">&larr; dashboard</a>
</div>
<x-status />

@php $select = 'w-full bg-cyber-900 border border-cyber-600 rounded-lg px-3 py-2.5 text-sm text-white focus:outline-none focus:border-cyber-neon'; @endphp
<form method="POST" action="{{ route('admin.binaries.store') }}" enctype="multipart/form-data" class="mb-8 grid gap-4 border border-cyber-600 bg-cyber-800/90 p-5 md:grid-cols-5">
    @csrf
    <div>
        <x-label for="os">OS</x-label>
        <select name="os" id="os" class="{{ $select }}">
            @foreach (\App\Http\Controllers\DownloadController::OSES as $key => $label)<option value="{{ $key }}" @selected(old('os') === $key)>{{ $label }}</option>@endforeach
        </select>
        <x-input-error :messages="$errors->get('os')" />
    </div>
    <div>
        <x-label for="arch">Architecture</x-label>
        <select name="arch" id="arch" class="{{ $select }}">
            @foreach (['amd64', 'arm64'] as $arch)<option @selected(old('arch') === $arch)>{{ $arch }}</option>@endforeach
        </select>
        <x-input-error :messages="$errors->get('arch')" />
    </div>
    <div>
        <x-label for="version">Version</x-label>
        <x-input name="version" id="version" value="{{ old('version') }}" placeholder="1.2.3" required />
        <x-input-error :messages="$errors->get('version')" />
    </div>
    <div class="md:col-span-2">
        <x-label for="file">Binary</x-label>
        <input type="file" name="file" id="file" required class="w-full text-sm text-gray-300 file:mr-3 file:border-0 file:bg-cyber-700 file:px-3 file:py-2.5 file:font-mono file:text-xs file:text-gray-200">
        <x-input-error :messages="$errors->get('file')" />
    </div>
    <div class="md:col-span-5"><x-button type="submit"><i class="fa-solid fa-upload mr-2"></i>Upload (MD5 and SHA-256 are calculated)</x-button></div>
</form>

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
