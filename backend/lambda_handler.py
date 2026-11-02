"""
Entry point for the zip-packaged AWS Lambda function (FastAPIApp in
template.yaml), which runs the app through Mangum behind API Gateway.

Kept out of app.py so other hosts (Render, the container image) don't need
mangum installed.
"""

from mangum import Mangum

from backend.app import app

handler = Mangum(app)
