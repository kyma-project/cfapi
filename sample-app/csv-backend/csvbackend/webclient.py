import logging
import io
import os.path
import glob
import pandas
import requests

def send_frame(frame):
  buf = io.BytesIO()
  frame.to_csv(buf)
  res = requests.post('http://localhost:5000/upload', data=buf.getvalue() )
  print('Response:')
  print(f'{res.status_code} : {res.text}')
  

def main(root):
  fs = glob.glob("*.csv", root_dir=root)
  for f in fs:
    fcsv = os.path.join(root, f)
    print(f'Loading file {fcsv}')
    df = pandas.read_csv(fcsv, on_bad_lines="skip")
    send_frame(df)
 
if __name__ == "__main__":
  main('cleandata')