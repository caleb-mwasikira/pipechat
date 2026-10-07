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
                <a href="#features" class="border border-cyber-600 px-5 py-3 font-mono text-xs font-bold uppercase tracking-wider text-gray-300 transition-all hover:border-cyber-neon hover:text-cyber-neon">See what it does</a>
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

{{-- Each feature is laid out like the hero: words on one side, a terminal showing it on the other. --}}
<section id="features" class="relative py-10 sm:py-16">
    <div class="term-grid absolute inset-0 -z-10"></div>
    <div class="grid items-center gap-10 lg:grid-cols-2">
        <div class="space-y-6">
            <p class="font-mono text-xs uppercase tracking-[0.2em] text-cyber-neon">// shared music</p>
            <h2 class="font-mono text-4xl font-black leading-tight text-white sm:text-5xl">Listen to group music<span class="caret"></span></h2>
            <p class="max-w-md text-sm leading-relaxed text-gray-400">Everyone uploads songs to one shared library on the website. In the terminal, type /play and the whole library streams, on shuffle or repeat. Songs you have heard once are kept on your computer, so they play again instantly.</p>
            <ul class="space-y-2 font-mono text-xs text-gray-300">
                <li class="flex items-start gap-2.5"><span class="text-cyber-neon">&gt;</span><span>/play, /shuffle, /repeat, /next and /stop</span></li>
                <li class="flex items-start gap-2.5"><span class="text-cyber-neon">&gt;</span><span>Name a song by its number, name or hash</span></li>
                <li class="flex items-start gap-2.5"><span class="text-cyber-neon">&gt;</span><span>/save keeps any song as a file</span></li>
            </ul>
        </div>
        <x-window title="pipechat &middot; music" accent="cyan" class="corners">
            <p class="term-line" style="--d:.1s"><span class="text-cyber-neon">&gt;</span> /play</p>
            <p class="term-line text-gray-400" style="--d:.5s">&#9834; 1. Midnight Drive &middot; Kora</p>
            <p class="term-line" style="--d:.9s"><span class="text-cyber-neon">&gt;</span> /shuffle on</p>
            <p class="term-line text-gray-400" style="--d:1.3s">shuffle is on</p>
            <p class="term-line" style="--d:1.7s"><span class="text-cyber-neon">&gt;</span> /save 3<span class="caret"></span></p>
        </x-window>
    </div>
</section>

<section class="relative py-10 sm:py-16">
    <div class="term-grid absolute inset-0 -z-10"></div>
    <div class="grid items-center gap-10 lg:grid-cols-2">
        <x-window title="pipechat &middot; files" class="corners order-2 lg:order-1">
            <p class="term-line" style="--d:.1s"><span class="text-cyber-neon">&gt;</span> /upload ~/Pictures/trip</p>
            <p class="term-line text-gray-400" style="--d:.5s">42 files &middot; 118 MB &middot; Enter to upload</p>
            <p class="term-line" style="--d:.9s"><span class="text-cyber-cyan">&#9608;&#9608;&#9608;&#9608;&#9608;&#9608;&#9608;&#9608;&#9608;&#9608;</span> 100%</p>
            <p class="term-line text-gray-400" style="--d:1.3s">shared as trip &middot; md5 9f2c&hellip;</p>
            <p class="term-line" style="--d:1.7s"><span class="text-cyber-neon">&gt;</span> /download trip ~/Downloads<span class="caret"></span></p>
        </x-window>
        <div class="order-1 space-y-6 lg:order-2">
            <p class="font-mono text-xs uppercase tracking-[0.2em] text-cyber-neon">// file sharing</p>
            <h2 class="font-mono text-4xl font-black leading-tight text-white sm:text-5xl">Share files from your shell</h2>
            <p class="max-w-md text-sm leading-relaxed text-gray-400">Send a file or a whole folder to a chat with /upload. It is packed and encrypted on your computer before it leaves, with a preview first and a progress bar while it goes. The other side checks its MD5 when it arrives.</p>
            <ul class="space-y-2 font-mono text-xs text-gray-300">
                <li class="flex items-start gap-2.5"><span class="text-cyber-neon">&gt;</span><span>Tab completes file names as you type</span></li>
                <li class="flex items-start gap-2.5"><span class="text-cyber-neon">&gt;</span><span>Every file has an MD5 hash you can compare</span></li>
                <li class="flex items-start gap-2.5"><span class="text-cyber-neon">&gt;</span><span>/download by name, number or the start of the hash</span></li>
            </ul>
        </div>
    </div>
</section>

<section class="relative py-10 sm:py-16">
    <div class="term-grid absolute inset-0 -z-10"></div>
    <div class="grid items-center gap-10 lg:grid-cols-2">
        <div class="space-y-6">
            <p class="font-mono text-xs uppercase tracking-[0.2em] text-cyber-neon">// get started</p>
            <h2 class="font-mono text-4xl font-black leading-tight text-white sm:text-5xl">Ready? It takes a minute<span class="caret"></span></h2>
            <p class="max-w-md text-sm leading-relaxed text-gray-400">Make an account, download the client for your system, log in with the same email and password, and you are chatting.</p>
            <div class="flex flex-wrap gap-3">
                @guest<a href="{{ route('register') }}" class="bg-cyber-neon px-5 py-3 font-mono text-xs font-bold uppercase tracking-wider text-black transition-all hover:bg-emerald-400 neon-glow-green"><i class="fa-solid fa-user-plus mr-2"></i>Create an account</a>@endguest
                <a href="{{ route('download') }}" class="border border-cyber-600 px-5 py-3 font-mono text-xs font-bold uppercase tracking-wider text-gray-300 transition-all hover:border-cyber-neon hover:text-cyber-neon"><i class="fa-solid fa-download mr-2"></i>Download</a>
            </div>
        </div>
        <x-window title="pipechat" class="corners">
            <p class="term-line" style="--d:.1s"><span class="text-cyber-neon">$</span> ./pipechat</p>
            <p class="term-line text-gray-400" style="--d:.5s">server: pipechat.kibandaskimart.com</p>
            <p class="term-line" style="--d:.9s">email: you@example.com</p>
            <p class="term-line text-gray-400" style="--d:1.3s">welcome back. type /help</p>
            <p class="term-line" style="--d:1.7s"><span class="text-cyber-neon">&gt;</span><span class="caret"></span></p>
        </x-window>
    </div>
</section>
@endsection
