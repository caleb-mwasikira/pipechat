@extends('layouts.app')
@section('title', 'Profile')
@section('content')
<div class="mb-8">
    <p class="font-mono text-xs uppercase tracking-[0.2em] text-cyber-neon">// profile</p>
    <h1 class="font-mono text-3xl font-black text-white">Your profile</h1>
    <p class="mt-2 text-sm text-gray-400">What other people see when they type <span class="font-mono text-cyber-cyan">/whois {{ '@'.$user->handle }}</span> in the terminal. You can also change your bio there with <span class="font-mono text-cyber-cyan">/bio TEXT</span>.</p>
</div>
<x-status />

<form method="POST" action="{{ route('profile.update') }}" class="max-w-xl space-y-4 border border-cyber-600 bg-cyber-800/90 p-5">
    @csrf
    @method('PUT')
    <div>
        <x-label>Handle</x-label>
        <p class="font-mono text-sm text-white">{{ '@'.$user->handle }} <span class="text-xs text-gray-500">(cannot be changed)</span></p>
    </div>
    <div>
        <x-label for="name">Name</x-label>
        <x-input icon="fa-solid fa-user" name="name" id="name" value="{{ old('name', $user->name) }}" required />
        <x-input-error :messages="$errors->get('name')" />
    </div>
    <div>
        <x-label for="bio">Bio</x-label>
        <textarea name="bio" id="bio" rows="4" maxlength="280" placeholder="A line or two about you" class="w-full bg-cyber-900 border border-cyber-600 rounded-lg px-4 py-2.5 text-sm text-white focus:outline-none focus:border-cyber-neon transition-all">{{ old('bio', $user->bio) }}</textarea>
        <p class="mt-1 font-mono text-[11px] text-gray-500">Up to 280 characters.</p>
        <x-input-error :messages="$errors->get('bio')" />
    </div>
    <x-button type="submit"><i class="fa-solid fa-floppy-disk mr-2"></i>Save</x-button>
</form>
@endsection
