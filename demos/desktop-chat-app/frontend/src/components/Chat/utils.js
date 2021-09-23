
export function getCurrentEmojiWord(startPos, text) {
    if (
        text[startPos] === " " ||
        text[startPos] === ":" ||
        startPos < 2
    ) {
        setEmojiSearchWord('')
        return
    }

    let cursor = startPos
    let searchWord = []

    while (true) {
        if (text[cursor] === ":" && cursor === 0) {
            setEmojiSearchWord(searchWord.reverse().join(''))
            break;
        }

        if (text[cursor] === ":" && text[cursor - 1] !== " ") {
            setEmojiSearchWord('')
            break;
        }

        if (cursor === 0 || text[cursor] === " ") {
            setEmojiSearchWord('')
            break;
        }

        if (
            text[cursor] === ":" &&
            text[cursor - 1] === " " &&
            searchWord.length >= 2
        ) {
            setEmojiSearchWord(searchWord.reverse().join(''))
            break;
        }

        if (text[cursor] !== ":") {
            searchWord.push(text[cursor].toLowerCase())
        }
        cursor--
    }

    return searchWord
}
