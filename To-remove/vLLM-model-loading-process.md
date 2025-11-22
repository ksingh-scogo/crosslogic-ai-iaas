
✅ 1. Direct Answer to Your Question (Authentication + vLLM)

❗ Your Worker is PROTECTED using access/secret keys

vLLM cannot natively pass:
	•	Authorization headers
	•	Basic Auth
	•	HMAC signed headers
	•	Bearer tokens
	•	Custom cookies

vLLM only supports plain HTTP/HTTPS fetching via fsspec.

Therefore:

⭐ vLLM CANNOT access private authenticated Workers.

⭐ You must make the Worker endpoint PUBLIC, but SECURE in another way.

This is how every AI infra provider does it.

⸻

✔️ Best Production Strategy: Signed URLs

You modify your Worker to check a signed URL or JWT instead of static credentials.

vLLM loads models like:

https://models.crosslogic.ai/llama/model-00001-of-00005.safetensors?sig=ABC123&exp=1713333923

This URL is:
	•	Publicly fetchable
	•	But only valid for N hours
	•	Cryptographically protected
	•	No access/secret keys needed in vLLM

⭐ vLLM downloads the model using HTTP → Range requests → CDN caching remains fully active.
⭐ Your R2 bucket stays 100% private.
⭐ Zero credentials leaked.

⸻

🧠 Why signed URLs are required

vLLM uses:

fsspec.filesystem("http")

which only supports:
	•	GET
	•	HEAD
	•	Range: bytes=…

It CANNOT send custom auth headers.

Signed URLs are the industry standard for loading private model weights:
	•	OpenAI
	•	Anthropic
	•	Together.ai
	•	Mistral
	•	MosaicML

All do this.

⸻

⭐ 2. PRODUCTION-GRADE PRD (MARKDOWN VERSION)

Below is a full engineering-ready PRD you can give your team.

⸻

📘 PRD: vLLM Model Loading From Cloudflare R2 via CDN (Private Bucket + Worker + Signed URLs)

1. Overview

We need to serve large LLM models (2GB–200GB+) using vLLM without local disk storage.
The model must be streamed directly via HTTP from Cloudflare CDN, not from R2 directly.

Key goals:
	•	R2 bucket must remain private
	•	CDN must serve all model files via HTTP Range requests
	•	vLLM must stream files without downloading to disk
	•	CDN should hold models hot-cached for 1 year
	•	No credentials must be embedded in vLLM

We achieve this via:

R2 (private) → Cloudflare Worker (signed URLs + Range support) → CDN → vLLM streamer


⸻

2. Requirements

Functional Requirements
	1.	vLLM must load model files directly from CDN URLs.
	2.	Worker must authenticate the request using signed URL tokens, not headers.
	3.	R2 must remain private and unexposed.
	4.	CDN must support:
	•	Range requests
	•	Large object streaming (up to 5GB shards)
	•	1-year caching
	5.	System must support multi-GB models split into HF-style shards.
	6.	vLLM must not require local storage.

Non-functional Requirements
	•	Latency: Requests should hit CDN (HIT) globally.
	•	Scalability: Support hundreds of workers loading models simultaneously.
	•	Security: No static credentials shipped to inference nodes.

⸻

3. Architecture

3.1 High-Level Diagram

                          +----------------+
                          |     vLLM       |
                          | (HTTP streamer)|
                          +--------+-------+
                                   |
                    HTTP(S)  Range | Requests
                                   v
                  +-------------------------------+
                  |      Cloudflare CDN POP       |
                  +------------------+------------+
                                     |
                          Cache MISS  | Cache HIT
                                     v
                         +--------------------+
                         | Cloudflare Worker  |
                         |  (auth + range)    |
                         +----------+---------+
                                    |
                             R2 Private Bucket


⸻

4. Cloudflare Worker Specification

4.1 Worker Responsibilities
	•	Validate signed URL token (sig, exp)
	•	Handle Range requests from vLLM
	•	Stream objects from R2
	•	Add long-term caching headers

4.2 Example Worker (Production-Ready)

export default {
  async fetch(request, env) {
    const url = new URL(request.url);
    const key = url.pathname.slice(1);

    // Validate signed URL
    const sig = url.searchParams.get("sig");
    const exp = Number(url.searchParams.get("exp"));

    if (!sig || !exp || Date.now() / 1000 > exp) {
      return new Response("Unauthorized", { status: 401 });
    }

    const expectedSig = await sign(env.SIGNING_SECRET, key, exp);
    if (sig !== expectedSig) {
      return new Response("Forbidden", { status: 403 });
    }

    // Handle Range request
    const range = request.headers.get("range");

    const object = await env.BUCKET.get(key, { range });

    if (!object) {
      return new Response("Not Found", { status: 404 });
    }

    return new Response(object.body, {
      status: range ? 206 : 200,
      headers: {
        "Content-Type": object.httpMetadata?.contentType || "application/octet-stream",
        ...(object.range ? {"Content-Range": object.range} : {}),
        "Cache-Control": "public, max-age=31536000, s-maxage=31536000"
      }
    });
  }
};

// HMAC helper
async function sign(secret, key, exp) {
  const encoder = new TextEncoder();
  const data = encoder.encode(`${key}:${exp}`);
  const cryptoKey = await crypto.subtle.importKey(
    "raw", encoder.encode(secret), { name: "HMAC", hash: "SHA-256" }, false, ["sign"]
  );
  const signature = await crypto.subtle.sign("HMAC", cryptoKey, data);
  return btoa(String.fromCharCode(...new Uint8Array(signature)));
}


⸻

5. Authentication Model

Because vLLM cannot send headers, the Worker must authenticate via:

5.1 Signed URL Token

https://models.crosslogic.ai/model/model-00001-of-00005.safetensors?
    sig=XXXXX
    &exp=1713333923

	•	sig = HMAC(key, exp)
	•	exp = unix timestamp expiry
	•	key = object path

5.2 Token Generator (Backend)

Backend generates URLs before launching vLLM:

def generate_signed_url(file_name):
    exp = int(time.time()) + 3600  # 1 hour
    msg = f"{file_name}:{exp}"

    sig = base64.b64encode(
        hmac.new(SIGNING_SECRET.encode(), msg.encode(), hashlib.sha256).digest()
    ).decode()

    return f"https://models.crosslogic.ai/{file_name}?sig={sig}&exp={exp}"


⸻

6. vLLM Model Loading

6.1 vLLM loads HF-style remote models via fsspec

Directory structure in R2 must mirror:

/model/
   config.json
   tokenizer.json
   model.safetensors.index.json
   model-00001-of-00005.safetensors
   model-00002-of-00005.safetensors
   ...

6.2 vLLM command

vllm serve \
  https://models.crosslogic.ai/model/?sig=ABC&exp=XYZ \
  --download-dir /dev/shm

vLLM will:
	1.	Fetch index.json
	2.	Fetch each shard with Range requests
	3.	Store chunks in RAM
	4.	Load into GPU memory

⸻

7. CDN Optimization

Enable in Cloudflare:
	•	Cache Everything
	•	Edge TTL = 1 year
	•	Serve Stale While Revalidate
	•	Cache Reserve enabled

Worker headers must include:

Cache-Control: public, max-age=31536000, s-maxage=31536000


⸻

8. Security Considerations
	•	Objects never become public (R2 stays private)
	•	Signed URLs expire
	•	Signing secret stored in Worker secrets
	•	vLLM nodes never store credentials
	•	Tokens can be short-lived (1 hour)
	•	Access logs auditable via Cloudflare Analytics

⸻

9. Performance Considerations
	•	Cloudflare CDN supports HTTP/3 + QUIC → fastest model fetches
	•	Range requests allow multi-GB streaming without RAM spikes
	•	Cache Reserve keeps shards hot
	•	Reduce shard count for optimal load times

⸻

10. Future Enhancements
	•	Pre-warm CDN POPs
	•	Multi-CDN replication (Cloudflare + AWS CloudFront)
	•	Automatic index.json generation
	•	Model versioning
