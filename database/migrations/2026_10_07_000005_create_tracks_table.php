<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        // The shared music library. The chat server reads this table and the files (storage/app/private/music)
        // to let people play tracks from the terminal.
        Schema::create('tracks', function (Blueprint $table) {
            $table->id();
            $table->string('title', 120);
            $table->string('artist', 120)->nullable();
            $table->string('path');            // under storage/app/private
            $table->string('extension', 5);
            $table->unsignedBigInteger('size');
            $table->foreignId('uploaded_by')->nullable()->constrained('users')->nullOnDelete();
            $table->timestamps();
        });
    }

    public function down(): void
    {
        Schema::dropIfExists('tracks');
    }
};
