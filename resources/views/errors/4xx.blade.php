@extends('errors.layout', ['code' => $exception->getStatusCode(), 'title' => 'Request not accepted', 'message' => 'That request could not be handled. Go back and try something else.'])
