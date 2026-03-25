So our Clerk auth implementation fell drastically short.

ISSUES WE EXPERIENCED:
- Clerk forces their own UI into our application, it breaks the unique feel and touch of our app
- Clerk felt very vendor-in like
- Clerk has upgrade to PRO features which is not what I want when I'm making a cloud application

What I liked about Clerk:
- Easy auth support: it was easy to drop-in and play.
- The DX was modern and nice

what I truly hated about clerk:
- I mentioned it above already, but there is more
- For every UI that felt short, I had to write my own and that felt very hacky
- I would be forced to use their SDKs and upgrade to PRO for more features.
- They limit Organizations to X amount of users, unless you upgrade.

Here is what I am thinking of doing next:
- Something minimal, but production-ready.
- Handles JWT, Multiple session sign-in, SSO, good security
- Doesn't have a vendor lock-in
- Can scale
- It has a decent DX
- Doesn't force prebuilt UIs
- Don't have to pay for PRO
- Is cheap and maintainable at scale

What would be nice to have, but not extremely important:
- Sending emails
- Organizations

Zitadel
