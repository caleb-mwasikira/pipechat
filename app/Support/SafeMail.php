<?php

namespace App\Support;

use App\Models\User;
use Illuminate\Contracts\Mail\Mailable;
use Illuminate\Support\Facades\Mail;
use Throwable;

class SafeMail
{
    /** Send a mail to a user. A mail server problem is logged but never breaks the page the person is on. */
    public static function to(User $user, Mailable $mail): void
    {
        try {
            Mail::to($user)->send($mail);
        } catch (Throwable $e) {
            report($e);
        }
    }
}
