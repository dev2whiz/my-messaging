# Gateway (Go) - DEPRECATED

> **Status:** Deprecated  
> **Replaced By:** `../gateway-spring`

This module contains the original Go implementation corresponding to Stage 1 (Foundation) of the messaging platform. It successfully provides JWT authentication, Valkey blocklists, PostgreSQL message persistence with cursor pagination, and RabbitMQ durable queues bounded to WebSockets.

It has been formally renamed and preserved as a reference, as the primary architecture has been renewed into Java 21+ Spring Boot to fully exploit Virtual Threads and Spring AMQP. Do not add new features here.
