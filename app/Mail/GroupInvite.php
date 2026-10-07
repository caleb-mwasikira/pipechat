<?php

namespace App\Mail;

use App\Models\User;
use Illuminate\Mail\Mailable;
use Illuminate\Mail\Mailables\Content;
use Illuminate\Mail\Mailables\Envelope;

class GroupInvite extends Mailable
{
    public function __construct(public User $inviter, public string $group) {}

    public function envelope(): Envelope
    {
        return new Envelope(subject: "@{$this->inviter->handle} invited you to {$this->group}");
    }

    public function content(): Content
    {
        return new Content(view: 'emails.group-invite', text: 'emails.text.group-invite');
    }
}
