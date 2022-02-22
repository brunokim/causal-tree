
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
        this.selection = [0, 0]
    }

    textKeyup(evt) {
        let textarea = $(evt.target)
        let content = textarea.val()
        let selection = [textarea.prop("selectionStart"), textarea.prop("selectionEnd")]

        if (content != this.content) {
            console.log(`${this.id}: ${this.content} -> ${content}`)
        }
        if (selection[0] != this.selection[0] || selection[1] != this.selection[1]) {
            // Selection range changed
            console.log(`${this.id}: ${this.selection[0]}:${this.selection[1]} -> ${selection[0]}:${selection[1]}`)
        }
        if (this.content == content &&
            this.selection[0] == selection[0] &&
            this.selection[1] == selection[1]) {
            return
        }
        fetch('/edit', {
            'method': 'POST',
            'body': new URLSearchParams({
                'id': this.id,
                'sel0T0': this.selection[0],
                'sel1T0': this.selection[1],
                'contentT0': this.content,
                'sel0T1': selection[0],
                'sel1T1': selection[1],
                'contentT1': content,
            }),
        }).then(this.handleEditResponse.bind(this)).catch(err => console.log(err))
        this.selection = selection
        this.content = content
    }

    handleEditResponse(response) {
        console.log('handleEditResponse')
        console.log(response)
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
