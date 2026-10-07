@extends('layouts.app')
@section('container', 'max-w-6xl')
@section('content')
<section class="relative py-10 sm:py-16">
    <div class="term-grid absolute inset-0 -z-10"></div>
    <div class="grid items-center gap-10 lg:grid-cols-2">
        <div class="space-y-6">
            <p class="font-mono text-xs uppercase tracking-[0.2em] text-cyber-neon">// terminal chat</p>
            <h1 class="font-mono text-4xl font-black leading-tight text-white sm:text-5xl">Chat from your terminal<span class="caret"></span></h1>
            <p class="max-w-md text-sm leading-relaxed text-gray-400">Pipechat is an end-to-end encrypted chat client that lives in your shell. Create an account, grab the binary for Linux or Windows, and connect.</p>
            <div class="flex flex-wrap gap-3">
                <a href="{{ route('download') }}" class="bg-cyber-neon px-5 py-3 font-mono text-xs font-bold uppercase tracking-wider text-black transition-all hover:bg-emerald-400 neon-glow-green"><i class="fa-solid fa-download mr-2"></i>Download</a>
                @guest<a href="{{ route('login') }}" class="border border-cyber-600 px-5 py-3 font-mono text-xs font-bold uppercase tracking-wider text-gray-300 transition-all hover:border-cyber-neon hover:text-cyber-neon">Login</a>@endguest
            </div>
        </div>
        <x-window title="pipechat" class="corners">
            <p class="term-line" style="--d:.1s"><span class="text-cyber-neon">$</span> ./pipechat-linux-amd64</p>
            <p class="term-line text-gray-400" style="--d:.5s">connected to chat server</p>
            <p class="term-line" style="--d:.9s"><span class="text-cyber-cyan">alice</span> &gt; anyone up for a deploy?</p>
            <p class="term-line" style="--d:1.3s"><span class="text-cyber-purple">bob</span> &gt; always. ship it.</p>
            <p class="term-line" style="--d:1.7s"><span class="text-cyber-neon">$</span><span class="caret"></span></p>
        </x-window>
    </div>
</section>
@endsection
