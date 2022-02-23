import {diff} from './diff.js'

export class CrdtController {
    constructor(container) {
        this.id = crypto.randomUUID()
        this.view = $("<div>")
            .addClass("crdt")
            .append($("<textarea>")
                .on('input', this.textInput.bind(this)))
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

    textInput(evt) {
        let textarea = $(evt.target)
        let content = textarea.val()

        if (content == this.content) {
            return
        }
        console.log(diff(this.content, content))

        console.log(`${this.content} -> ${content}`)
        fetch('/edit', {
            'method': 'POST',
            'body': new URLSearchParams({
                'id': this.id,
                'contentT0': this.content,
                'contentT1': content,
            }),
        }).then(this.handleEditResponse.bind(this)).catch(err => console.log(err))
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

        let textarea1 = $("textarea", this.view).first();
        let textarea2 = $("textarea", sibling.view).first();

        // Copy properties of this textarea to forked textarea.
        textarea2.val(textarea1.val())
        textarea2.prop('selectionStart', textarea1.prop('selectionStart'))
        textarea2.prop('selectionEnd', textarea1.prop('selectionEnd'))
        textarea2.prop('selectionDirection', textarea1.prop('selectionDirection'))


        // TODO: implement /fork endpoint.
        // For now, treat all text as new from scratch.
        sibling.textInput({target: textarea2})
    }
}
