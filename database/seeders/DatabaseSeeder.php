<?php

namespace Database\Seeders;

use App\Models\User;
use Illuminate\Database\Seeder;
use Illuminate\Support\Str;

class DatabaseSeeder extends Seeder
{
    public const ADMIN_EMAIL = 'germanchefhard@gmail.com';

    /**
     * Seed the application's database. Safe to run again: it never changes an
     * existing admin's password unless ADMIN_PASSWORD is set.
     */
    public function run(): void
    {
        $admin = User::firstOrNew(['email' => self::ADMIN_EMAIL]);
        $password = env('ADMIN_PASSWORD');

        if (! $admin->exists) {
            $password ??= Str::password(20, symbols: false);
            $this->command?->warn("Admin account created: ".self::ADMIN_EMAIL." / $password  (change it via Forgot password)");
        }

        $admin->name = $admin->name ?: 'Admin';
        $admin->handle ??= 'admin';
        if ($password) {
            $admin->password = $password;
        }
        $admin->email_verified_at ??= now();
        $admin->is_admin = true;
        $admin->save();
    }
}
