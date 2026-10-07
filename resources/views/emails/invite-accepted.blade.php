<x-mail.layout title="{{ '@'.$member->handle }} joined {{ $group }}" preheader="Your invitation was accepted." button="Open your groups" :url="route('groups')">
    <p style="margin:0 0 14px 0;"> <strong style="color:#00ff66;">{{ '@'.$member->handle }}</strong> accepted your invitation to <strong style="color:#ffffff;">{{ $group }}</strong>.</p>
    <p style="margin:0 0 14px 0;"> Keep your terminal open for a moment: it shares the group's key with them automatically, and the group then appears in their terminal.</p>
</x-mail.layout>
