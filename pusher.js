var conn, sendMessage;

$(function() {
	conn = new WebSocket("ws://localhost:8080/pusher");
	var frames = $('iframe');

	frames.on('load', function() {
		console.log('loaded, will swap');
		swap();
	});

	conn.onclose = function(evt) {
		console.log("onclose", evt);
	};

	conn.onopen = function(evt) {
		console.log("onopen", evt);
	};

	conn.onerror = function(evt) {
		console.log("onerror", evt);
	};

	conn.onmessage = function(evt) {
		console.log("onmessage", evt);

		var message = JSON.parse(evt.data);
		if (message.type == "url") {
			loadURL(message.payload);
		}
	};

	sendMessage = function(type, payload) {
		console.log("Sending msg", type, payload);
		conn.send(JSON.stringify({ "type": type, "payload": payload }));
	};

	function currentFrame() {
		return $('iframe.current');
	}

	function nextFrame() {
		return $('iframe.next');
	}

	function swap() {
		var current = currentFrame();
		var next = nextFrame();

		next.removeClass('loading');
		current.removeClass('current'); current.addClass('next');
		next.removeClass('next'); next.addClass('current');
		console.log('swapped');
	}

	function loadURL(url) {
		var next = nextFrame();
		next.addClass('loading');
		next.attr('src', url);
	}
});
