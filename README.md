# IPRanges
List all IP ranges of some companies, cloud services, and popular sites. This project is a considerably extended fork of [IPRanges](https://github.com/lord-alfred/ipranges) by [Lord_Alfred](https://t.me/Lord_Alfred).
I maintain this for my personal use on my home router mostly. 

## Included ip address ranges
### Cloud Services and Content Delivery Networks
Amazon (AWS), Alibaba Cloud, Azure (Microsoft), Backblaze, Digital Ocean, Google Cloud, Hetzner, Linode, MTS Cloud, Oracle, Yandex Cloud, Akamai, CacheFly, Cloudflare, EdgeCast, Fastly, Imperva, Qrator

### Search Engines, Social Media, and Communication Platforms
Bing (Microsoft), Google, Yahoo, Yandex, Facebook, LinkedIn, TikTok, Twitter (X.com), Vkontakte, YouTube, Kinopub, Rambler

### E-commerce, Technology Companies, and Online Services
Alibaba, Amazon, Avito, Ozon.ru, Adobe, Apple, Microsoft, Sberbank, Hugging Face, GitHub

### Internet Service Providers, VPNs, and Regional Services
ExpressVPN, NordVPN, ProtonVPN, Surfshark, Popular VPN services servers, Apple's iCloud Private Relay, Beeline, Corbina, M247, MTS, Rostelecom, Russian Government sites

## Repository Structure
	.
	├── [service_name]/
	│   ├── downloader.sh
	│   ├── ipv4.txt
	│   ├── ipv6.txt
	│   ├── ipv4_merged.txt
	│   └── ipv6_merged.txt
	├── utils/
	│   └── [python scripts for IP processing]
	└── .github/
	    └── workflows/
	        └── update.yaml
### Service Folders
Each service folder contains:

- `downloader.sh`: Script for fetching IP ranges from public sources
- `ipv4.txt`/`ipv6.txt`: the list of addresses (IPv4 or IPv6), which is the result of parsing one or more sources
- `ipv4_merged.txt`/`ipv6_merged.txt`: optimized lists combined into the smallest possible CIDRs
 
### Utility Scripts

The `utils/` folder houses Python scripts for merging and cleaning IP lists.

### Automated Updates
Daily updates are managed via GitHub Actions, as defined in `.github/workflows/update.yaml`.

## Sister repos
Here's list of another data I use for my router configs:
  * [ipnames](https://github.com/antonme/ipnames): daily updated list of FQDNs and resolved IPs of some popular sites/platforms. Made for routing of some VPNs in my router
  * [geoip](https://github.com/antonme/geoip): lists of CIDR's by regions for routing VPNs in my router
