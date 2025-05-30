#!/usr/bin/env python3
import json
import random
import sys
from datetime import datetime, timedelta

# Helper functions
def generate_patient_id(i):
    return f"P{i:05d}"

def generate_device_id(i):
    return f"D{random.randint(10000,99999)}"  # Random device id, e.g., D12345

def random_timestamp():
    # Generate a timestamp within the last 30 days.
    now = datetime.now()
    delta = timedelta(
        days=random.randint(0, 30),
        hours=random.randint(0, 23),
        minutes=random.randint(0, 59),
        seconds=random.randint(0, 59)
    )
    ts = now - delta
    return ts.strftime("%Y-%m-%dT%H:%M:%S")

# Function to calculate the size of a record (excluding patient ID)
def calculate_data_size(record_data):
    # Convert the dictionary to a JSON string and get the size
    record_data_str = json.dumps(record_data)
    return len(record_data_str.encode('utf-8'))

# Function to generate the actual data with dynamically adjusted content
def generate_data(target_size):
    # Generate plausible random values
    heart_rate = f"{random.randint(60, 100)} bpm"
    systolic = random.randint(110, 130)
    diastolic = random.randint(70, 90)
    blood_pressure = f"{systolic}/{diastolic} mmHg"
    oxygen_saturation = f"{random.randint(95, 100)}%"
    respiratory_rate = f"{random.randint(12, 20)} breaths/min"
    steps = random.randint(1000, 10000)
    activity_level = random.choice(["Sedentary", "Light", "Moderate", "Intense"])
    sleep_duration = f"{random.randint(6, 9)} hours"
    glucose_level = f"{random.randint(80, 110)} mg/dL"
    temperature = f"{round(random.uniform(36.5, 37.5), 1)}°C"

    data = {
        "heart_rate": heart_rate,
        "blood_pressure": blood_pressure,
        "oxygen_saturation": oxygen_saturation,
        "respiratory_rate": respiratory_rate,
        "steps": steps,
        "activity_level": activity_level,
        "sleep_duration": sleep_duration,
        "glucose_level": glucose_level,
        "temperature": temperature
    }

    # Calculate the size of the generated data
    current_size = calculate_data_size(data)
    # Adjust the values in the data to fit the target size
    while current_size > target_size:
        # Randomly shorten or modify one of the fields to reduce the size
        key = random.choice(list(data.keys()))
        if isinstance(data[key], str):
            data[key] = data[key][:len(data[key]) - 1]  # Shorten a string by 1 character
        elif isinstance(data[key], int):
            data[key] = data[key] - 1  # Decrease an integer value by 1

        current_size = calculate_data_size(data)

    return data

def generate_record(i, target_size=870):
    record = {
        "patient_id": generate_patient_id(i),
        "device_id": generate_device_id(i),
        "timestamp": random_timestamp(),
        "data": generate_data(target_size - len(generate_patient_id(i).encode('utf-8'))),  # Ensure total size does not exceed target size
        "location": random.choice(["Home", "Clinic", "Hospital"])
    }
    return record

def main():
    if len(sys.argv) < 2:
        print("Usage: python generate_iot_data.py <num_records>")
        sys.exit(1)
    try:
        num_records = int(sys.argv[1])
    except ValueError:
        print("Invalid number provided. Please enter an integer.")
        sys.exit(1)
    
    records = []
    for i in range(1, num_records + 1):
        records.append(generate_record(i))
    
    output_file = "iot_medical_data.json"
    with open(output_file, "w") as f:
        json.dump(records, f, indent=4)
    
    print(f"Generated {num_records} IoT medical data records in {output_file}")

if __name__ == "__main__":
    main()
