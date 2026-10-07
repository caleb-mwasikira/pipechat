<x-mail.layout title="Welcome, {{ $user->name }}" preheader="Your account is ready. Download the terminal client." button="Download the client" :url="route('download')">
    <p style="margin:0 0 14px 0;"> Your account is ready. Your chat handle is <strong style="color:#00ff66;">{{ '@'.$user->handle }}</strong>.</p>
    <p style="margin:0 0 14px 0;"> Log in to the terminal client with this email address and your password:</p>
    <pre style="margin:0 0 14px 0;padding:12px 14px;background-color:#000000;border:1px solid #1f2937;color:#00f0ff;font-size:12px;"><span style="color:#00ff66;">$</span> ./pipechat-linux-amd64
email:    {{ $user->email }}
password: ••••••••</pre>
    <p style="margin:0 0 14px 0;"> Make groups and invite friends on the website; they show up in your terminal.</p>
</x-mail.layout>
