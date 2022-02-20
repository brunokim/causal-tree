
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

        this.container = container
        this.container.append(this.view)

        this.controllers = []
        this.content = ""
        this.selStart = 0
        this.selEnd = 0
    }

    textKeyup(evt) {
        let textarea = $(evt.target)
        let selStart = textarea.prop("selectionStart")
        let selEnd = textarea.prop("selectionEnd")
        let content = textarea.val()

        if (content != this.content) {
            console.log(`${this.id}: ${this.content} -> ${content}`)
        } else if (selStart == selEnd) {
            // Cursor changed
            if (selStart != this.selStart) {
                console.log(`${this.id}: ${this.selStart} -> ${selStart}`)
            }
        } else {
            // Selection range changed
            if (selStart != this.selStart || selEnd != this.selEnd) {
                console.log(`${this.id}: ${this.selStart}:${this.selEnd} -> ${selStart}:${selEnd}`)
            }
        }
        this.selStart = selStart
        this.selEnd = selEnd
        this.content = content
    }

    sync() {
        console.log('sync')
    }

    fork() {
        let sibling = new CrdtController(this.container)
        this.controllers.push(sibling)
        sibling.controllers.push(this)

        let siblingTextarea = $("textarea", sibling.view).first();
        siblingTextarea.val(this.content)
        siblingTextarea.prop("selectionStart", this.selStart);
        siblingTextarea.prop("selectionEnd", this.selEnd);
        sibling.textKeyup({target: siblingTextarea})
    }
}
