<?php

namespace App\Mail;

use App\Models\User;
use Illuminate\Mail\Mailable;
use Illuminate\Mail\Mailables\Content;
use Illuminate\Mail\Mailables\Envelope;

class Welcome extends Mailable
{
    public function __construct(public User $user) {}

    public function envelope(): Envelope
    {
        return new Envelope(subject: 'Welcome to '.config('app.name'));
    }

    public function content(): Content
    {
        return new Content(view: 'emails.welcome', text: 'emails.text.welcome');
    }
}
