<?php

namespace App\Providers;

use Illuminate\Auth\Notifications\ResetPassword;
use Illuminate\Notifications\Messages\MailMessage;
use Illuminate\Support\ServiceProvider;

class AppServiceProvider extends ServiceProvider
{
    /**
     * Register any application services.
     */
    public function register(): void
    {
        //
    }

    /**
     * Bootstrap any application services.
     */
    public function boot(): void
    {
        // The password reset email uses the site's own look.
        ResetPassword::toMailUsing(fn ($user, string $token) => (new MailMessage)
            ->subject('Reset your '.config('app.name').' password')
            ->view(['emails.reset-password', 'emails.text.reset-password'], [
                'url' => route('password.reset', ['token' => $token, 'email' => $user->getEmailForPasswordReset()]),
                'minutes' => config('auth.passwords.'.config('auth.defaults.passwords').'.expire'),
            ]));
    }
}
