# Import requests for making API calls
import requests

# Import netaddr for manipulating CIDRs
import netaddr

# Import sys for accessing command line arguments
import sys

# Import os and re for the optional shadow-run response cache
import os
import re

# Check if an org_handle was provided as a command line argument
if len(sys.argv) > 1:
    # Get the org_handle from the first argument
    org_handle = sys.argv[1]
else:
    # Print an error message and exit if no org_handle was given
    print("Please provide an organization handle as a command line argument.")
    sys.exit(1)

# Define the base URL for ARIN's Whois-RWS API
base_url = "https://whois.arin.net/rest/"

# Define the headers for requesting XML data
headers = {"Accept": "application/xml"}

# Fetch a URL, replaying or recording it in the irrindex shadow-run response
# cache when SHADOW_RESPONSE_CACHE_DIR is set (same scheme as its curl shim:
# successes are cached, failures are kept only for the paired run)
def fetch(url):
    cache_dir = os.environ.get("SHADOW_RESPONSE_CACHE_DIR")
    if not cache_dir:
        response = requests.get(url, headers=headers)
        return response.status_code, response.text
    key = re.sub(r"[^A-Za-z0-9._-]", "_", url)
    cache_file = os.path.join(cache_dir, "curl", key)
    if os.path.isfile(cache_file):
        with open(cache_file) as f:
            return 200, f.read()
    failure_dir = os.environ.get("SHADOW_RUN_FAILURE_DIR")
    failed_file = os.path.join(failure_dir, "curl", key) if failure_dir else None
    if failed_file and os.path.isfile(failed_file + ".status"):
        with open(failed_file + ".status") as f:
            status_code = int(f.read())
        if status_code == 0:
            raise requests.ConnectionError(f"replayed failure for {url}")
        return status_code, ""
    try:
        response = requests.get(url, headers=headers)
    except requests.RequestException:
        record_failure(failed_file, 0)
        raise
    if response.status_code != 200:
        record_failure(failed_file, response.status_code)
        return response.status_code, response.text
    os.makedirs(os.path.dirname(cache_file), exist_ok=True)
    tmp_file = f"{cache_file}.tmp.{os.getpid()}"
    with open(tmp_file, "w") as f:
        f.write(response.text)
    os.replace(tmp_file, cache_file)
    return response.status_code, response.text

# Remember a failed fetch so the paired shadow run sees the same outcome
def record_failure(failed_file, status_code):
    if not failed_file:
        return
    os.makedirs(os.path.dirname(failed_file), exist_ok=True)
    with open(failed_file + ".status", "w") as f:
        f.write(str(status_code))

# Make a GET request to get the networks for the organization
status_code, xml_data = fetch(base_url + "org/" + org_handle + "/nets")

# Check if the response status code is 200 (OK)
if status_code == 200:
    # Initialize an empty list for storing CIDRs
    cidrs = []

    # Loop through each network element in the XML data
    for net in xml_data.split("<netRef ")[1:]:
        # Extract the startAddress and endAddress attributes
        start_address = net.split('startAddress="')[1].split('"')[0]
        end_address = net.split('endAddress="')[1].split('"')[0]

        # Convert them to IP addresses using netaddr.IPAddress()
        start_ip = netaddr.IPAddress(start_address)
        end_ip = netaddr.IPAddress(end_address)

        # Create an IP range using netaddr.IPRange()
        ip_range = netaddr.IPRange(start_ip, end_ip)

        # Convert it to a list of CIDRs using cidrs() method
        ip_cidrs = ip_range.cidrs()

        # Append each CIDR to the cidrs list
        for cidr in ip_cidrs:
            cidrs.append(cidr)

    # Print out the number and list of CIDRs
    print("\n".join(str(cidr) for cidr in cidrs))

else:
    # Print out an error message if something went wrong
    print(f"Something went wrong. Status code: {status_code}")
