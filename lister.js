var conn, sendMessage;

$(function() {
	function connect() {
		var reconnectTimer;
		var host = 'ws://' + window.location.host + window.location.pathname.replace(/\/[^\/]+$/, '/') + 'pusher';

		conn = new WebSocket(host);

		conn.onclose = function(evt) {
			console.log("onclose", evt);

			// Set a reconnect timer...
			if (reconnectTimer) {
				clearTimeout(reconnectTimer);
			}

			reconnectTimer = setTimeout(connect, 3000);
		};

		conn.onopen = function(evt) {
			console.log("onopen", evt);
			sendMessage("list");

			$('a.create').click(function() {
				sendMessage("set", {});
			});
		};

		conn.onerror = function(evt) {
			console.log("onerror", evt);
		};

		conn.onmessage = function(evt) {
			console.log("onmessage", evt.data);

			var message = JSON.parse(evt.data);

			if (message.Type == "list") {
				console.log(message);
				listEntries(message.Payload);
			}
		};
	}

	function listEntries(entries) {
		var container = $('#directory');
		container.children().remove();
		for (var i = 0; i < entries.length; i++) {
			var entry =$('<li/>')
				.append($('<input>', {value: entries[i].ID, class: 'id', type: 'hidden'}))
				.append($('<input>', {value: entries[i].Name, placeholder: 'Name', class: 'name'}))
				.append($('<input>', {value: entries[i].Type, placeholder: 'Type', class: 'type'}))
				.append($('<input>', {value: entries[i].DumbDuration, placeholder: 'Duration', class: 'duration'}))
				.append($('<input>', {value: entries[i].URL, placeholder: 'URL', class: 'url'}))
				.append($('<a>', {text: "Destroy", class: 'destroy'}))

			entry.find('input').keypress(function(e) {
				if (e.which != 13) {
					return;
				}
				var inputs = $(this).parent().find('input');
				var params = {
					ID: $(this).parent().find('.id').val(),
					Type: $(this).parent().find('.type').val(),
					URL: $(this).parent().find('.url').val(),
					DumbDuration: $(this).parent().find('.duration').val(),
					Name: $(this).parent().find('.name').val()
				};
				sendMessage("set", params);
			});

			entry.find('a.destroy').click(function() {
				sendMessage("delete", $(this).parent().find('.id').val());
			});

			container.append(entry);
		}
	}

	sendMessage = function(type, payload) {
		conn.send(JSON.stringify({ "type": type, "payload": payload }));
	};

	connect();
}());
