<x-mail.layout title="You're invited to {{ $group }}" preheader="{{ '@'.$inviter->handle }} invited you to a group." button="View invitation" :url="route('groups')">
    <p style="margin:0 0 14px 0;"> <strong style="color:#00ff66;">{{ '@'.$inviter->handle }}</strong> invited you to join the group <strong style="color:#ffffff;">{{ $group }}</strong>.</p>
    <p style="margin:0 0 14px 0;"> Accept or decline on the website. If you accept, the group appears in your terminal chat once another member is online to share its key.</p>
</x-mail.layout>
