
export class CrdtController {
    constructor(container) {
        this.id = crypto.randomUUID()
        this.view = $("<div>")
            .addClass("crdt")
            .append($("<textarea>")
                .on('keyup', this.textKeyup.bind(this)))
            .append($("<button>")
                .append("Sync")
                .click(this.sync.bind(this)))
            .append($("<button>")
                .append("Fork")
                .click(this.fork.bind(this)))

        this.content = ""
        this.selStart = 0
        this.selEnd = 0

        container.append(this.view)
    }

    textKeyup(evt) {
        let textarea = $(evt.target)
        let selStart = textarea.prop("selectionStart")
        let selEnd = textarea.prop("selectionEnd")
        let content = textarea.val()

        if (selStart == selEnd) {
            if (selStart != this.selStart) {
                console.log(`${this.selStart} -> ${selStart}`)
            }
        } else {
            if (selStart != this.selStart || selEnd != this.selEnd) {
                console.log(`${this.selStart}:${this.selEnd} -> ${selStart}:${selEnd}`)
            }
        }
        if (content != this.content) {
            console.log(this.content, "->", content)
        }
        this.selStart = selStart
        this.selEnd = selEnd
        this.content = content
    }

    sync(evt) {
        console.log('sync')
        console.log(evt)
    }

    fork(evt) {
        console.log('fork')
        console.log(evt)
    }
}
