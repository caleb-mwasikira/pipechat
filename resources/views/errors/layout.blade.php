<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{ $code }} {{ $title }} // {{ config('app.name') }}</title>
    <style>html{background:#030712}body{visibility:hidden}html.show body{visibility:visible!important}</style>
    <noscript><style>body{visibility:visible}</style></noscript>
    <script>setTimeout(function(){document.documentElement.classList.add('show')},2000)</script>
    <link rel="icon" href="/favicon.svg" type="image/svg+xml">
    {{-- An error page must never fail itself: no routes, no database, no session, and styles only if they were built. --}}
    @if (file_exists(public_path('build/manifest.json')))
        @vite(['resources/css/app.css'])
    @endif
</head>
<body class="flex min-h-screen flex-col items-center justify-center bg-cyber-900 px-4 py-10 selection:bg-cyber-neon selection:text-black">
    <div class="fixed inset-0 scanlines z-50 pointer-events-none"></div>
    <div class="term-grid fixed inset-0 -z-10"></div>

    <main class="w-full max-w-xl space-y-6">
        <a href="/" class="flex items-center gap-2.5" aria-label="{{ config('app.name') }} home">
            <span class="flex h-9 w-9 items-center justify-center border border-cyber-neon bg-cyber-800 font-mono text-sm font-black text-cyber-neon">&gt;_</span>
            <span class="font-mono text-lg font-black tracking-tight text-white">{{ config('app.name') }}<span class="text-cyber-neon">.</span></span>
        </a>

        <div class="corners border border-cyber-600 bg-cyber-800 shadow-2xl">
            <div class="flex items-center gap-2 border-b border-cyber-600 bg-cyber-700/60 px-3 py-2">
                <span class="h-2.5 w-2.5 bg-red-400/80"></span><span class="h-2.5 w-2.5 bg-amber-300/80"></span><span class="h-2.5 w-2.5 bg-cyber-neon"></span>
                <span class="ml-2 truncate font-mono text-[11px] text-gray-400">pipechat &middot; error {{ $code }}</span>
            </div>
            <div class="space-y-1 p-5 font-mono text-[13px] leading-relaxed">
                <p class="term-line" style="--d:.1s"><span class="text-cyber-neon">$</span> open {{ e(request()->path() === '/' ? '/' : '/'.request()->path()) }}</p>
                <p class="term-line text-red-400" style="--d:.5s">error {{ $code }}: {{ strtolower($title) }}</p>
                <p class="term-line text-gray-400" style="--d:.9s">{{ $message }}</p>
                <p class="term-line" style="--d:1.3s"><span class="text-cyber-neon">$</span><span class="caret"></span></p>
            </div>
        </div>

        <div class="space-y-4">
            <p class="font-mono text-6xl font-black text-white sm:text-7xl">{{ $code }}</p>
            <h1 class="font-mono text-xl font-black text-white sm:text-2xl">{{ $title }}</h1>
            <p class="max-w-md text-sm leading-relaxed text-gray-400">{{ $message }}</p>
            <div class="flex flex-wrap gap-3 pt-2">
                <a href="/" class="bg-cyber-neon px-5 py-3 font-mono text-xs font-bold uppercase tracking-wider text-black transition-all hover:bg-emerald-400 neon-glow-green">Go home</a>
                <a href="javascript:history.length > 1 ? history.back() : (location.href = '/')" class="border border-cyber-600 px-5 py-3 font-mono text-xs font-bold uppercase tracking-wider text-gray-300 transition-all hover:border-cyber-neon hover:text-cyber-neon">Go back</a>
            </div>
        </div>
    </main>
</body>
</html>
