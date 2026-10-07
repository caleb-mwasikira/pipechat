<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <meta name="csrf-token" content="{{ csrf_token() }}">
    <title>@hasSection('title')@yield('title') // @endif{{ config('app.name') }} // Terminal chat</title>
    {{-- Keep the page hidden (on the theme's dark background) until app.css has applied, so unstyled HTML never shows. app.css sets body back to visible. --}}
    <style>html{background:#030712}body{visibility:hidden}html.show body{visibility:visible!important}</style>
    <noscript><style>body{visibility:visible}</style></noscript>
    <script>setTimeout(function(){document.documentElement.classList.add('show')},3000)</script>
    <link rel="icon" href="/favicon.svg" type="image/svg+xml">
    <link rel="icon" href="/favicon.ico" sizes="48x48">
    <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.4.0/css/all.min.css">
    @vite(['resources/css/app.css', 'resources/js/app.js'])
</head>
<body class="min-h-screen bg-cyber-900 flex flex-col relative selection:bg-cyber-neon selection:text-black">
    <div class="fixed inset-0 scanlines z-50 pointer-events-none"></div>

    <div class="hidden border-b border-cyber-700/60 bg-cyber-900 sm:block">
        <div class="mx-auto flex h-7 w-full max-w-screen-2xl items-center justify-between px-4 font-mono text-[10px] uppercase tracking-[0.2em] text-gray-500 sm:px-6 lg:px-8">
            <span class="flex items-center gap-2"><span class="h-1.5 w-1.5 animate-pulse bg-cyber-neon"></span>secure channel &middot; online</span>
            <span>end-to-end encrypted</span>
        </div>
    </div>

    <header class="sticky top-0 z-40 border-b border-cyber-600/60 bg-cyber-900/85 backdrop-blur-md">
        <div class="mx-auto flex h-16 w-full max-w-screen-2xl items-center justify-between gap-4 px-4 sm:px-6 lg:px-8">
            <a href="{{ route('home') }}" class="group flex shrink-0 items-center gap-2.5" aria-label="{{ config('app.name') }} home">
                <span class="flex h-9 w-9 shrink-0 items-center justify-center border border-cyber-neon bg-cyber-800 font-mono text-sm font-black text-cyber-neon transition-all group-hover:bg-cyber-neon group-hover:text-black">&gt;_</span>
                <span class="font-mono text-lg font-black tracking-tight text-white">{{ config('app.name') }}<span class="text-cyber-neon">.</span></span>
            </a>

            <div class="hidden shrink-0 items-center gap-3 md:flex">
                @php $navClass = fn ($active) => 'whitespace-nowrap border-b-2 px-2 py-5 font-mono text-xs font-bold uppercase tracking-wider transition-colors '.($active ? 'border-cyber-neon text-cyber-neon' : 'border-transparent text-gray-400 hover:text-white'); @endphp
                <a href="{{ route('download') }}" class="{{ $navClass(request()->routeIs('download')) }}">Download</a>
                @auth
                    <a href="{{ route('dashboard') }}" class="{{ $navClass(request()->routeIs('dashboard', 'groups', 'groups.*', 'music', 'profile', 'admin', 'admin.*')) }}">Dashboard</a>
                @endauth
                @guest
                    <a href="{{ route('login') }}" class="whitespace-nowrap border border-cyber-600 px-3 py-2 font-mono text-xs font-bold uppercase tracking-wider text-gray-300 transition-all hover:border-cyber-neon hover:text-cyber-neon">Login</a>
                    <a href="{{ route('register') }}" class="whitespace-nowrap bg-cyber-neon px-3 py-2 font-mono text-xs font-bold uppercase tracking-wider text-black transition-all hover:bg-emerald-400 neon-glow-green">Register</a>
                @endguest
            </div>

            <button type="button" data-drawer-open aria-label="Open menu" aria-controls="drawer" aria-expanded="false"
                    class="flex h-10 w-10 items-center justify-center border border-cyber-600 bg-cyber-800 text-gray-300 transition-all hover:border-cyber-neon hover:text-cyber-neon md:hidden">
                <i class="fa-solid fa-bars"></i>
            </button>
        </div>
    </header>

    {{-- Mobile drawer. It sits outside the blurred header, so `fixed` is relative to the screen. --}}
    <div id="drawer" class="pointer-events-none fixed inset-0 z-[60] md:hidden" aria-hidden="true">
        <div data-drawer-close class="absolute inset-0 bg-black/70 opacity-0 backdrop-blur-sm transition-opacity duration-300"></div>
        <div data-drawer-panel class="absolute right-0 top-0 flex h-full w-72 max-w-[85vw] translate-x-full flex-col border-l border-cyber-600 bg-cyber-900 shadow-2xl transition-transform duration-300">
            <div class="flex h-16 shrink-0 items-center justify-between border-b border-cyber-600/60 px-4">
                <span class="font-mono text-sm font-black text-white"><span class="mr-2 text-cyber-neon">&gt;_</span>{{ config('app.name') }}<span class="text-cyber-neon">.</span></span>
                <button type="button" data-drawer-close aria-label="Close menu" class="h-9 w-9 text-gray-400 transition-all hover:bg-cyber-700 hover:text-cyber-neon"><i class="fa-solid fa-xmark text-base"></i></button>
            </div>
            <div class="flex-1 overflow-y-auto">
                @php $drawerItem = 'flex items-center gap-2.5 border-l-2 px-4 py-3 font-mono text-xs font-bold transition-all '; @endphp
                @auth
                    @include('partials.dashboard-menu')
                @else
                    <nav class="py-3">
                        <a href="{{ route('home') }}" class="{{ $drawerItem }} {{ request()->routeIs('home') ? 'border-cyber-neon bg-cyber-700/60 text-cyber-neon' : 'border-transparent text-gray-300 hover:border-cyber-neon hover:text-white' }}"><i class="fa-solid fa-house w-4 text-center"></i>Home</a>
                        <a href="{{ route('download') }}" class="{{ $drawerItem }} {{ request()->routeIs('download') ? 'border-cyber-neon bg-cyber-700/60 text-cyber-neon' : 'border-transparent text-gray-300 hover:border-cyber-neon hover:text-white' }}"><i class="fa-solid fa-download w-4 text-center"></i>Download</a>
                    </nav>
                @endauth
            </div>
            @guest
                <div class="shrink-0 space-y-2 border-t border-cyber-600/60 p-4">
                    <a href="{{ route('login') }}" class="block border border-cyber-neon/50 px-4 py-2.5 text-center font-mono text-xs font-bold uppercase tracking-wider text-cyber-neon transition-all hover:bg-cyber-neon hover:text-black"><i class="fa-solid fa-right-to-bracket mr-1.5"></i>Login</a>
                    <a href="{{ route('register') }}" class="block bg-cyber-neon px-4 py-2.5 text-center font-mono text-xs font-bold uppercase tracking-wider text-black transition-all hover:bg-emerald-400 neon-glow-green"><i class="fa-solid fa-user-plus mr-1.5"></i>Register</a>
                </div>
            @endguest
        </div>
    </div>

    @auth
        @if (request()->routeIs('groups', 'groups.*', 'music', 'profile', 'admin', 'admin.*'))
            <div class="mx-auto flex w-full max-w-screen-2xl flex-grow">
                <aside class="hidden w-60 shrink-0 border-r border-cyber-700 bg-cyber-800/40 md:block">
                    <div class="sticky top-16">@include('partials.dashboard-menu')</div>
                </aside>
                <main class="min-w-0 flex-1 px-3 py-8 sm:px-6 lg:px-8">@yield('content')</main>
            </div>
        @else
            <main class="flex-grow @yield('container', 'max-w-5xl') mx-auto px-3 sm:px-6 lg:px-8 py-8 w-full">@yield('content')</main>
        @endif
    @else
        <main class="flex-grow @yield('container', 'max-w-5xl') mx-auto px-3 sm:px-6 lg:px-8 py-8 w-full">@yield('content')</main>
    @endauth

    <footer class="border-t border-cyber-600/60 bg-cyber-900">
        <div class="mx-auto flex w-full max-w-screen-2xl flex-col items-center justify-between gap-2 px-4 py-6 font-mono text-[11px] text-gray-500 sm:flex-row sm:px-6 lg:px-8">
            <span>&copy; {{ date('Y') }} {{ config('app.name') }}</span>
            <span><span class="text-cyber-neon">$</span> exit 0</span>
        </div>
    </footer>
</body>
</html>
