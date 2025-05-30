#!/usr/bin/env python3
import os
import base64
import json
import sys

def generate_random_token():
    # 618 random bytes will yield exactly 824 characters when Base64 encoded.
    random_bytes = os.urandom(618)
    token = base64.b64encode(random_bytes).decode("utf-8")
    return token

def main():
    if len(sys.argv) < 2:
        print("Usage: python generate_encrypted_data_json.py <num_tokens>")
        sys.exit(1)
    try:
        num_tokens = int(sys.argv[1])
    except ValueError:
        print("Invalid number provided. Please enter an integer.")
        sys.exit(1)
    
    data = []
    for _ in range(num_tokens):
        token = generate_random_token()
        data.append( {"data": token})
    
    output_file = "encrypted_data.json"
    with open(output_file, "w") as f:
        json.dump(data, f, indent=4)
    
    print(f"Generated {num_tokens} encrypted tokens in {output_file}")

if __name__ == "__main__":
    main()
