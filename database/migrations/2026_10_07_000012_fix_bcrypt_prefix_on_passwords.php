<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Support\Facades\DB;

return new class extends Migration
{
    public function up(): void
    {
        // The chat server used to write passwords as $2a$, which Laravel refuses to check ("does not use the Bcrypt
        // algorithm"). It is the same algorithm spelled differently, so the accounts made or reset from the terminal
        // are repaired by changing the prefix.
        DB::statement("UPDATE users SET password = '\$2y\$' || substr(password, 5) WHERE password LIKE '\$2a\$%'");
    }

    public function down(): void
    {
        // Nothing to undo: $2y$ is what Laravel writes itself.
    }
};
