
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

export function isPDF(mimetype) {
    switch (mimetype.toLowerCase()) {
    case 'application/pdf':
        return true
    default:
        return false
    }
}