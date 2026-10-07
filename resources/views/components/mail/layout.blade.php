@props(['title', 'preheader' => '', 'button' => null, 'url' => null])
{{-- Email layout in the site's terminal look. Table-based with inline styles, because mail clients ignore most CSS. --}}
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="color-scheme" content="dark light">
<title>{{ $title }}</title>
</head>
<body style="margin:0;padding:0;background-color:#030712;">
<div style="display:none;max-height:0;overflow:hidden;opacity:0;color:#030712;">{{ $preheader }}</div>
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" bgcolor="#030712" style="background-color:#030712;">
<tr><td align="center" style="padding:32px 16px;">
  <table role="presentation" width="560" cellpadding="0" cellspacing="0" border="0" style="width:100%;max-width:560px;">
    <tr><td style="padding:0 0 20px 0;font-family:'JetBrains Mono','Courier New',monospace;">
      <span style="display:inline-block;border:1px solid #00ff66;background-color:#0b0f19;color:#00ff66;font-size:14px;font-weight:bold;padding:7px 9px;">&gt;_</span>
      <span style="font-size:20px;font-weight:bold;color:#ffffff;vertical-align:middle;padding-left:8px;">{{ config('app.name') }}<span style="color:#00ff66;">.</span></span>
    </td></tr>
    <tr><td bgcolor="#0b0f19" style="background-color:#0b0f19;border:1px solid #1f2937;border-top:2px solid #00ff66;padding:32px 28px;font-family:'JetBrains Mono','Courier New',monospace;color:#e5e7eb;font-size:14px;line-height:1.7;">
      <h1 style="margin:0 0 18px 0;font-size:20px;line-height:1.3;color:#ffffff;">{{ $title }}</h1>
      {{ $slot }}
      @if ($button && $url)
        <table role="presentation" cellpadding="0" cellspacing="0" border="0" style="margin:26px 0 8px 0;"><tr>
          <td bgcolor="#00ff66" style="background-color:#00ff66;">
            <a href="{{ $url }}" style="display:inline-block;padding:13px 24px;font-family:'JetBrains Mono','Courier New',monospace;font-size:13px;font-weight:bold;letter-spacing:1px;text-transform:uppercase;color:#000000;text-decoration:none;">{{ $button }}</a>
          </td>
        </tr></table>
        <p style="margin:18px 0 0 0;font-size:11px;line-height:1.6;color:#6b7280;">Button not working? Copy this link into your browser:<br><a href="{{ $url }}" style="color:#00f0ff;word-break:break-all;">{{ $url }}</a></p>
      @endif
    </td></tr>
    <tr><td style="padding:20px 4px 0 4px;font-family:'JetBrains Mono','Courier New',monospace;font-size:11px;line-height:1.6;color:#6b7280;">
      <span style="color:#00ff66;">$</span> {{ config('app.name') }} &middot; end-to-end encrypted terminal chat<br>
      You are getting this because of activity on your {{ config('app.name') }} account.
    </td></tr>
  </table>
</td></tr>
</table>
</body>
</html>
