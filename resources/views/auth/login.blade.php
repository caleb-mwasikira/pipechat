@extends('layouts.app')
@section('title', 'Login')
@section('content')
<x-card>
    <form method="POST" action="{{ route('login') }}" class="space-y-4">
        @csrf
        <x-heading title="SECURE ACCOUNT LOGIN" sub="Enter your credentials to access downloads." />
        <div>
            <x-label for="email">Email</x-label>
            <x-input icon="fa-solid fa-at" type="email" name="email" id="email" value="{{ old('email') }}" required autofocus />
            <x-input-error :messages="$errors->get('email')" />
        </div>
        <div>
            <div class="flex items-center justify-between mb-1.5">
                <x-label for="password" class="!mb-0">Password</x-label>
                <a href="{{ route('password.request') }}" class="text-[11px] text-gray-400 hover:text-cyber-neon transition-colors">Forgot password?</a>
            </div>
            <x-password-input name="password" id="password" required />
            <x-input-error :messages="$errors->get('password')" />
        </div>
        <label class="flex items-center space-x-2 text-xs text-gray-300">
            <input type="checkbox" name="remember" class="w-4 h-4 rounded bg-cyber-900 border-cyber-600 text-cyber-neon focus:ring-0">
            <span>Remember me</span>
        </label>
        <x-button type="submit"><i class="fa-solid fa-terminal mr-2"></i>Sign In</x-button>
        <div class="text-center"><a href="{{ route('register') }}" class="text-[11px] text-gray-400 hover:text-cyber-neon transition-colors">Don't have an account? Register.</a></div>
    </form>
</x-card>
@endsection
