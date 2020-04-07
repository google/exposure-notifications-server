import os

from flask import Flask
import redis


app = Flask(__name__)


redis_host = os.environ.get('REDIS_HOST', 'localhost')
redis_port = int(os.environ.get('REDIS_PORT', 6379))
redis_password = os.environ.get('REDIS_PASSWORD', None)
redis_client = redis.StrictRedis(
    host=redis_host, port=redis_port, password=redis_password)


@app.route('/')
def index():
    value = redis_client.incr('counter', 1)
    return 'Value is {}'.format(value)


if __name__ == '__main__':
    # This is used when running locally only. When deploying to Google App
    # Engine, a webserver process such as Gunicorn will serve the app. This
    # can be configured by adding an `entrypoint` to app.yaml.
    app.run(host='127.0.0.1', port=8080, debug=True)
