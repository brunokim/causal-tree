import {diff} from './diff.js'

export class CrdtController {
    constructor(container) {
        this.id = crypto.randomUUID()
        this.view = $("<div>")
            .addClass("crdt")
            .append($("<textarea>")
                .on('input', evt => this.textInput(evt)))
            .append($("<button>")
                .append("Sync")
                .click(evt => this.sync(evt)))
            .append($("<button>")
                .append("Fork")
                .click(evt => this.fork(evt)))

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
        let ops = diff(this.content, content)
        this.content = content
        console.log(ops)

        let body = {'id': this.id, 'ops': ops}
        fetch('/edit', {
            'method': 'POST',
            'headers': {
                'Accept': 'application/json',
                'Content-Type': 'application/json',
            },
            'body': JSON.stringify(body),
        })
        .then(response => response.text())
        .then(text => this.handleEditResponse(text))
        .catch(err => console.log(err))
    }

    handleEditResponse(text) {
        console.log('handleEditResponse')
        console.log(text)
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
