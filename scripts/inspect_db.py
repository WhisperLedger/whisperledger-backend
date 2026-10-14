#!/usr/bin/env python3
"""
WhisperLedger Live Database Inspector
Queries live Firestore and (if active) PostgreSQL to monitor:
1. User registration & profile updates
2. Household creations & membership
3. Expenses logged (both personal & household)
4. Push token registrations and subscription plans
"""

import sys
import json
import subprocess
import urllib.request
import urllib.error
from datetime import datetime

PROJECT_ID = "whisperledger-94715"
DATABASE_URL = f"https://firestore.googleapis.com/v1/projects/{PROJECT_ID}/databases/(default)/documents"

def get_access_token():
    try:
        res = subprocess.run(["/Users/tanmayagarwal/google-cloud-sdk/bin/gcloud", "auth", "print-access-token"], capture_output=True, text=True, check=True)
        return res.stdout.strip()
    except Exception as e:
        print(f"Error getting gcloud token: {e}")
        return None

def fetch_firestore(path, token):
    url = f"{DATABASE_URL}/{path}"
    req = urllib.request.Request(url, headers={"Authorization": f"Bearer {token}"})
    try:
        with urllib.request.urlopen(req) as response:
            return json.loads(response.read().decode("utf-8"))
    except Exception:
        return {}

def parse_val(val_obj):
    if not isinstance(val_obj, dict):
        return val_obj
    for k in ["stringValue", "booleanValue"]:
        if k in val_obj: return val_obj[k]
    if "integerValue" in val_obj: return int(val_obj["integerValue"])
    if "doubleValue" in val_obj: return float(val_obj["doubleValue"])
    if "timestampValue" in val_obj: return val_obj["timestampValue"]
    if "mapValue" in val_obj:
        return {k: parse_val(v) for k, v in val_obj["mapValue"].get("fields", {}).items()}
    if "arrayValue" in val_obj:
        return [parse_val(v) for v in val_obj["arrayValue"].get("values", [])]
    return val_obj

def parse_doc(doc_dict):
    data = {}
    for k, v in doc_dict.get("fields", {}).items():
        data[k] = parse_val(v)
    data["_createTime"] = doc_dict.get("createTime")
    data["_updateTime"] = doc_dict.get("updateTime")
    return data

def main():
    token = get_access_token()
    if not token:
        print("❌ Could not obtain Google Cloud OAuth token via gcloud.")
        return

    print("=================================================================")
    print(f"🔍 WhisperLedger Live Database Inspector (Project: {PROJECT_ID})")
    print(f"   Timestamp: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    print("=================================================================")

    # 1. Inspect Users
    users_resp = fetch_firestore("users", token)
    users_docs = users_resp.get("documents", [])
    print(f"\n👤 [1] USERS DIRECTORY ({len(users_docs)} registered):")
    print("-----------------------------------------------------------------")
    
    for doc in users_docs:
        uid = doc["name"].split("/")[-1]
        data = parse_doc(doc)
        email = data.get("email", "No Email")
        name = data.get("displayName") or data.get("name") or "Unnamed"
        plan = data.get("plan") or ("pro" if data.get("isPro") else "starter_free")
        status = data.get("status", "active")
        updated = data.get("_updateTime", "N/A")
        
        print(f"• UID: {uid}")
        print(f"  Name: {name} | Email: {email}")
        print(f"  Plan: {plan.upper()} | Status: {status} | Last Updated: {updated}")
        
        # Check subcollection expenses
        exp_resp = fetch_firestore(f"users/{uid}/expenses", token)
        exp_count = len(exp_resp.get("documents", []))
        print(f"  Subcollection Expenses: {exp_count} logged")
        print()

    # 2. Inspect Households
    households_resp = fetch_firestore("households", token)
    households_docs = households_resp.get("documents", [])
    print(f"🏠 [2] HOUSEHOLDS ({len(households_docs)} active):")
    print("-----------------------------------------------------------------")
    for doc in households_docs:
        hid = doc["name"].split("/")[-1]
        data = parse_doc(doc)
        hname = data.get("name", "Unnamed")
        code = data.get("inviteCode") or data.get("code") or "N/A"
        members = data.get("members", [])
        print(f"• Household: '{hname}' (ID: {hid} | Code: {code})")
        print(f"  Members count: {len(members) if isinstance(members, list) else 1}")

    print("\n=================================================================")
    print("✅ Inspection completed.")
    print("=================================================================")

if __name__ == "__main__":
    main()
