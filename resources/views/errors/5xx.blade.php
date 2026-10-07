@extends('errors.layout', ['code' => $exception->getStatusCode(), 'title' => 'Server problem', 'message' => 'Something went wrong on our side. Try again in a moment.'])
