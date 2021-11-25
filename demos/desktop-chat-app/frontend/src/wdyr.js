/* eslint-disable */
import React from 'react'
if (process.env.NODE_ENV === 'development') {
    const whyDidYouRender = require('@welldone-software/why-did-you-render')
    whyDidYouRender(React, {
        titleColor: '#3F51B5',
        diffNameColor: '#FF5722',
        diffPathColor: '#FFC107',
        trackAllPureComponents: true,
        collapseGroups: true,
        include: [
            // Uncomment to see renders of all components
            // /.*/,
            // Add individual component names here
            // /^Sidebar/,
            // /^Attachment/,
            // /^ServerAndRoomInfo/,
            /^Redwood/,
        ],
    })
}
/* eslint-enable */
