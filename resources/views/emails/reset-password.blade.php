<x-mail.layout title="Reset your password" preheader="Use this link to choose a new password." button="Choose a new password" :url="$url">
    <p style="margin:0 0 14px 0;"> We got a request to reset the password for your account.</p>
    <p style="margin:0 0 14px 0;"> This link works for {{ $minutes }} minutes. If you did not ask for it, ignore this email; your password stays the same.</p>
    <p style="margin:0 0 14px 0;"> <span style="color:#f59e0b;">Heads up:</span> your terminal client's saved keys are protected by your password. After a reset it starts with new keys, and your groups come back as other members come online.</p>
</x-mail.layout>
