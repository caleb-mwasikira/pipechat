<?php

use App\Http\Controllers\AdminBinaryController;
use App\Http\Controllers\AdminController;
use App\Http\Controllers\AuthController;
use App\Http\Controllers\DownloadController;
use App\Http\Controllers\GroupController;
use App\Http\Controllers\InviteLinkController;
use App\Http\Controllers\MusicController;
use App\Http\Controllers\PasswordResetController;
use App\Http\Controllers\ProfileController;
use Illuminate\Support\Facades\Route;

Route::view('/', 'home')->name('home');
Route::get('/download', [DownloadController::class, 'index'])->name('download');
Route::get('/download/{binary}', [DownloadController::class, 'show'])->whereNumber('binary')->name('download.file');

Route::middleware('guest')->group(function () {
    Route::view('/register', 'auth.register')->name('register');
    Route::post('/register', [AuthController::class, 'register'])->middleware('throttle:10,1');
    Route::view('/login', 'auth.login')->name('login');
    Route::post('/login', [AuthController::class, 'login'])->middleware('throttle:10,1');

    Route::view('/forgot-password', 'auth.forgot-password')->name('password.request');
    Route::post('/forgot-password', [PasswordResetController::class, 'sendLink'])
        ->middleware('throttle:5,1')->name('password.email');
    Route::get('/reset-password/{token}', fn (string $token) => view('auth.reset-password', ['token' => $token]))
        ->name('password.reset');
    Route::post('/reset-password', [PasswordResetController::class, 'reset'])->name('password.update');
});

Route::middleware('auth')->group(function () {
    Route::post('/logout', [AuthController::class, 'logout'])->name('logout');
    Route::get('/profile', [ProfileController::class, 'edit'])->name('profile');
    Route::put('/profile', [ProfileController::class, 'update'])->name('profile.update');
    Route::get('/music', [MusicController::class, 'index'])->name('music');
    Route::post('/music', [MusicController::class, 'store'])->middleware('throttle:300,10')->name('music.store');
    Route::get('/music/{track}/audio', [MusicController::class, 'audio'])->whereNumber('track')->name('music.audio');
    Route::delete('/music/{track}', [MusicController::class, 'destroy'])->whereNumber('track')->name('music.destroy');
    Route::redirect('/dashboard', '/groups')->name('dashboard');
    Route::get('/groups', [GroupController::class, 'index'])->name('groups');
    Route::post('/groups', [GroupController::class, 'store'])->name('groups.store');
    Route::post('/groups/{group}/join', [GroupController::class, 'join'])->whereNumber('group')->middleware('throttle:20,1')->name('groups.join');
    Route::post('/groups/{group}/invite-codes', [GroupController::class, 'createCode'])->whereNumber('group')->middleware('throttle:20,1')->name('groups.codes.create');
    Route::delete('/groups/{group}/invite-codes/{code}', [GroupController::class, 'revokeCode'])->whereNumber(['group', 'code'])->name('groups.codes.revoke');
    Route::post('/groups/{group}/invite-link', [GroupController::class, 'inviteLink'])->whereNumber('group')->name('groups.invite-link');
    Route::get('/join/{token}', [InviteLinkController::class, 'show'])->where('token', '[A-Za-z0-9]{20,40}')->name('invite.show');
    Route::post('/join/{token}', [InviteLinkController::class, 'join'])->where('token', '[A-Za-z0-9]{20,40}')->middleware('throttle:20,1')->name('invite.join');
    Route::post('/groups/{group}/visibility', [GroupController::class, 'visibility'])->whereNumber('group')->name('groups.visibility');
    Route::get('/groups/{group}', [GroupController::class, 'show'])->whereNumber('group')->name('groups.show');
    Route::put('/groups/{group}', [GroupController::class, 'rename'])->whereNumber('group')->name('groups.rename');
    Route::get('/groups/{group}/files', [GroupController::class, 'files'])->whereNumber('group')->name('groups.files');
    Route::get('/groups/{group}/music', [GroupController::class, 'music'])->whereNumber('group')->name('groups.music');
    Route::post('/groups/{group}/invite', [GroupController::class, 'invite'])->middleware('throttle:20,1')->name('groups.invite');
    Route::post('/groups/{group}/leave', [GroupController::class, 'leave'])->name('groups.leave');
    Route::post('/invites/{invite}/{answer}', [GroupController::class, 'respond'])->name('invites.respond');
    Route::get('/admin', [AdminController::class, 'index'])->middleware('admin')->name('admin');
    Route::middleware('admin')->prefix('admin')->group(function () {
        Route::get('/binaries', [AdminBinaryController::class, 'index'])->name('admin.binaries');
        Route::get('/binaries/upload', [AdminBinaryController::class, 'create'])->name('admin.binaries.create');
        Route::post('/binaries', [AdminBinaryController::class, 'store'])->name('admin.binaries.store');
        Route::delete('/binaries/{binary}', [AdminBinaryController::class, 'destroy'])->whereNumber('binary')->name('admin.binaries.destroy');
    });
});
