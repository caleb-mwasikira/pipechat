@extends('layouts.app')
@section('title', 'Upload binary')
@section('content')
<div class="mb-8">
    <p class="font-mono text-xs uppercase tracking-[0.2em] text-cyber-neon">// admin</p>
    <h1 class="font-mono text-3xl font-black text-white">Upload a binary</h1>
</div>

@php $select = 'w-full bg-cyber-900 border border-cyber-600 rounded-lg px-3 py-2.5 text-sm text-white focus:outline-none focus:border-cyber-neon'; @endphp
<form method="POST" action="{{ route('admin.binaries.store') }}" enctype="multipart/form-data" class="max-w-3xl grid gap-4 border border-cyber-600 bg-cyber-800/90 p-5 md:grid-cols-5">
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

@endsection
