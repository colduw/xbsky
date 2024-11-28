# xbsky
A simple Bluesky embed fixer for Telegram and Discord, written in Go

# Usage
Add an `x` before `bsky.app`, so it becomes `xbsky.app`

### Want the raw/direct media only?

Add `raw` before `xbsky.app`, so it becomes `raw.xbsky.app`

### A post has multiple images, but you want a combined one?

Add `mosaic` before `xbsky.app`, so it becomes `mosaic.xbsky.app`

# Gallery

<p>A text only post</p>
<img src="./docs/d_textpost.png">
<br>

<p>A text only post, it also works with a spoiler tag</p>
<img src="./docs/d_spoilerpost.png">
<br>

<p>A text only, reply post. It embeds the parent's image</p>
<img src="./docs/d_replyembed.png">
<br>

<p>A text only, reply post. It embeds the parent's video (text is cut off due to Discord limits, and replaced with <code>...</code>)</p>
<img src="./docs/d_replyandvideo.png">
<br>

<p>A text only post that has an external embed (in this case, a link to Twitch), it embeds external metadata (title, description, image if available)</p>
<img src="./docs/d_external.png">
<br>

<p>A profile embed</p>
<img src="./docs/d_profile.png">
<br>

<p>A profile embed, it also works with a spoiler tag</p>
<img src="./docs/d_spoilerprofile.png">
<br>

<p>A text only post with an external embed (Telegram)</p>
<img src="./docs/tg_external.png">
<br>

<p>A text only, quote post (Telegram)</p>
<img src="./docs/tg_quote.png">
<br>

<p>A reply post with two images, horizontally stacked for the image preview (Telegram only; all images are available in Instant View)</p>
<img src="./docs/tg_mosaic.png">
<br>

<p>A video post (Telegram)</p>
<img src="./docs/tg_video.png">
<br>

# Note
- This project was done as practice, if you encounter any bugs, errors, or whatnot, feel free to reach out to me:
    - On [Discord (@reallycoldunwanted)](https://discord.com/users/928010351583330414)
    - On [Bluesky (@coldunwanted.net)](https://bsky.app/profile/coldunwanted.net)
    - Or, of course, [here on GitHub](https://github.com/colduw/xbsky/issues)
