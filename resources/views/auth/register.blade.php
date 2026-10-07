@extends('layouts.app')
@section('title', 'Register')
@section('content')
<x-card icon="fa-user-shield">
    <form method="POST" action="{{ route('register') }}" class="space-y-4">
        @csrf
        <x-heading title="CREATE YOUR ACCOUNT" sub="Register to download the Pipechat terminal client." />
        <div>
            <x-label for="name">Name</x-label>
            <x-input icon="fa-solid fa-user" type="text" name="name" id="name" value="{{ old('name') }}" required autofocus />
            <x-input-error :messages="$errors->get('name')" />
        </div>
        <div>
            <x-label for="handle">Chat handle</x-label>
            <x-input icon="fa-solid fa-at" type="text" name="handle" id="handle" value="{{ old('handle') }}" placeholder="a-z, 0-9, _ (3-20)" required />
            <x-input-error :messages="$errors->get('handle')" />
        </div>
        <div>
            <x-label for="email">Email</x-label>
            <x-input icon="fa-solid fa-envelope" type="email" name="email" id="email" value="{{ old('email') }}" required />
            <x-input-error :messages="$errors->get('email')" />
        </div>
        <div>
            <x-label for="password">Password</x-label>
            <x-password-input icon="fa-solid fa-key" name="password" id="password" required />
            <x-input-error :messages="$errors->get('password')" />
        </div>
        <div>
            <x-label for="password_confirmation">Confirm Password</x-label>
            <x-password-input icon="fa-solid fa-key" name="password_confirmation" id="password_confirmation" required />
        </div>
        <x-button type="submit"><i class="fa-solid fa-fingerprint mr-2"></i>Create Account</x-button>
        <div class="text-center"><a href="{{ route('login') }}" class="text-[11px] text-gray-400 hover:text-cyber-neon transition-colors">Already have an account? Sign in.</a></div>
    </form>
</x-card>
@endsection
