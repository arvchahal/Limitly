
# Limitly
This repository explores the implementation and performance of rate-limiting algorithms, focusing on their impact when used in a **reverse proxy** setting. The study was conducted as part of **CS 8395 - Advanced Topics in Software Engineering**, with an emphasis on real-world scenarios involving varying system loads.

### Project Overview
- **Algorithms Explored:**
  - Token Bucket
  - Leaky Bucket
  - Fixed Window
  - Sliding Window
- **Core Features:**
  - **Reverse Proxy Functionality:** 
    - The rate-limiting algorithms were implemented to regulate traffic as a reverse proxy, ensuring controlled throughput while handling multiple client requests.
  - **Load Conditions:**
    - Evaluated performance under **burst loads** and **constant loads**, simulating real-world scenarios.
  - **Metrics Monitored:**
    - Power consumption.
    - Throughput under stress and normal operation.

### Implementation Details
- **Tech Stack:** Written in Go and deployed on Raspberry Pi devices for distributed testing.
- **Command-Line Configuration:**
  - Allows selection of rate-limiting algorithms and tuning of parameters such as token replenishment rates and burst capacities.
- **Distributed Setup:**
  - Server and client run on separate Raspberry Pi devices.
  - Algorithms run as independent services to simulate modular deployment.

### Report Availability
The final project report, detailing implementation, experiments, and findings, is available upon request. Please contact us if you are interested.

