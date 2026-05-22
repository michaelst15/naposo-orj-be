from a2wsgi import WSGIMiddleware

from server import app

application = WSGIMiddleware(app)
