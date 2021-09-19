function getExtension(filename) {
	let parts = filename.split('.');
	return parts[parts.length - 1];
}

export function isImage(mimetype) {
    switch (mimetype.toLowerCase()) {
    case 'image/jpeg':
    case 'image/jpg':
    case 'image/gif':
    case 'image/png':
    case 'image/tif':
    case 'image/tiff':
    case 'image/bmp':
        return true
    default:
        return false
    }
}

export function isAudio(mimetype) {
	switch (mimetype.toLowerCase()) {
		case 'audio/mpeg':
		case 'audio/mp4':
		case 'audio/vnd.wav':
		case 'audio/x-mpegurl':
			return true
		default:
			return false
	}
}

export function isVideo(filename) {
	let ext = getExtension(filename);
	switch (ext.toLowerCase()) {
		case 'm4v':
		case 'avi':
		case 'mpg':
		case 'mp4':
		case 'mov':
		return true;
	}
	return false;
  }

export function isPDF(mimetype) {
    switch (mimetype.toLowerCase()) {
    case 'application/pdf':
        return true
    default:
        return false
    }
}