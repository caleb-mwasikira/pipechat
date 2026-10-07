<?php

namespace App\Mail;

use App\Models\User;
use Illuminate\Mail\Mailable;
use Illuminate\Mail\Mailables\Content;
use Illuminate\Mail\Mailables\Envelope;

class InviteAccepted extends Mailable
{
    public function __construct(public User $member, public string $group) {}

    public function envelope(): Envelope
    {
        return new Envelope(subject: "@{$this->member->handle} joined {$this->group}");
    }

    public function content(): Content
    {
        return new Content(view: 'emails.invite-accepted', text: 'emails.text.invite-accepted');
    }
}
