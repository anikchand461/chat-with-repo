import os
from pathlib import Path

import requests
from dotenv import load_dotenv

# --------------------------------------------------
# Load Environment Variables
# --------------------------------------------------

print("=" * 60)
print("DODO STARTUP")
print("=" * 60)

print("Current working directory:", Path.cwd())
print("Using .env from:", Path(".env").resolve())

load_dotenv(override=True)

BASE_URL = "https://test.dodopayments.com"

API_KEY = os.getenv("DODO_API_KEY")
PRODUCT_ID = os.getenv("DODO_PRODUCT_ID")
APP_URL = os.getenv("APP_URL")

print("\nEnvironment Variables")
print("-" * 60)

if API_KEY:
    print("API KEY PREFIX :", repr(API_KEY[:12]))
    print("API KEY LENGTH :", len(API_KEY))
else:
    print("❌ DODO_API_KEY not found")

print("PRODUCT ID     :", PRODUCT_ID)
print("APP URL        :", APP_URL)

print("=" * 60)
print()


class DodoClient:

    def __init__(self):
        self.headers = {
            "Authorization": f"Bearer {API_KEY}",
            "Content-Type": "application/json",
        }

    # --------------------------------------------------
    # Create Checkout Session
    # --------------------------------------------------

    def create_checkout(
        self,
        email: str,
        user_id: int,
    ):

        payload = {
            "product_cart": [
                {
                    "product_id": PRODUCT_ID,
                    "quantity": 1,
                }
            ],
            "customer": {
                "email": email,
            },
            "metadata": {
                "user_id": str(user_id),
            },
            "return_url": f"{APP_URL}/payment/success",
        }

        print("\n========== DODO REQUEST ==========")
        print("POST:", f"{BASE_URL}/checkout/sessions")

        if API_KEY:
            print("Authorization: Bearer", API_KEY[:12] + "...")
        else:
            print("Authorization: No API Key")

        print("Payload:")
        print(payload)

        print("==================================")

        response = requests.post(
            f"{BASE_URL}/checkouts",
            headers=self.headers,
            json=payload,
            timeout=30,
        )

        print("\n========== DODO RESPONSE ==========")
        print("Status :", response.status_code)
        print("Body   :", response.text)
        print("===================================\n")

        response.raise_for_status()

        return response.json()

    # --------------------------------------------------
    # Get Subscription
    # --------------------------------------------------

    def get_subscription(self, subscription_id: str):

        response = requests.get(
            f"{BASE_URL}/subscriptions/{subscription_id}",
            headers=self.headers,
            timeout=30,
        )

        print("\nSubscription Status:", response.status_code)
        print(response.text)

        response.raise_for_status()

        return response.json()

    # --------------------------------------------------
    # Get Customer
    # --------------------------------------------------

    def get_customer(self, customer_id: str):

        response = requests.get(
            f"{BASE_URL}/customers/{customer_id}",
            headers=self.headers,
            timeout=30,
        )

        print("\nCustomer Status:", response.status_code)
        print(response.text)

        response.raise_for_status()

        return response.json()


dodo = DodoClient()