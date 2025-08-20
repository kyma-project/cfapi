import time
import logging
import io
import os
from flask import Flask
from flask import request
import pandas as pd

app = Flask(__name__)
logger = logging.getLogger('server')
VOLUME = os.environ.get('VOLUME') or '.'

@app.route('/')
def hello():
  return "Hello from csv backend"

@app.post('/upload')
def upload():
  df = pd.read_csv(io.BytesIO(request.data), encoding='utf8', on_bad_lines="skip")
  df.to_csv(f'{VOLUME}/{int(time.time())}.csv')
  return "all good"

def setup_log():
  logging.basicConfig(level=logging.INFO)
  
if __name__ == "__main__":
  setup_log()
  app.run()