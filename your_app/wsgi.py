from asgiref.wsgi import AsgiToWsgi

from server import app

application = AsgiToWsgi(app)
