import os


def positive_int(name, default):
    value = int(os.getenv(name, default))
    if value <= 0:
        raise ValueError(f"{name} must be a positive integer")
    return value


bind = "0.0.0.0:5000"
workers = positive_int("WEB_CONCURRENCY", 2)
worker_class = "gthread"
threads = positive_int("GUNICORN_THREADS", 2)
timeout = positive_int("GUNICORN_TIMEOUT_SECONDS", 30)
graceful_timeout = positive_int("GUNICORN_GRACEFUL_TIMEOUT_SECONDS", 10)
keepalive = 5
accesslog = "-"
errorlog = "-"
capture_output = True
control_socket_disable = True
