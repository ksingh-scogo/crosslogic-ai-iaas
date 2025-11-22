import requests
from requests.auth import HTTPBasicAuth

# Your Worker-secured CDN URL
LIST_URL = "https://models.crosslogic.ai/list"

# Your Cloudflare R2 Access/Secret keys
ACCESS_KEY = "06bed70271956ca5f9c4bee231dd17e7"
SECRET_KEY = "21dcf559a49adda4bcdf3dabb3ecf7d4481baf9fed9e31c6e759ed3eb25c13a9"

response = requests.get(
    LIST_URL,
    auth=HTTPBasicAuth(ACCESS_KEY, SECRET_KEY)
)

print("Status:", response.status_code)
print("Response:")
print(response.text)