const path = require('path')
const webpack = require('webpack')
const MiniCssExtractPlugin = require('mini-css-extract-plugin');
const OptimizeCSSAssetsPlugin = require('optimize-css-assets-webpack-plugin');
const safePostCssParser = require('postcss-safe-parser');
const postcssNormalize = require('postcss-normalize');
const paths = require('./paths');

const ProgressBarWebpackPlugin = require('progress-bar-webpack-plugin')

module.exports = {
    // Get mode from NODE_ENV
    mode: process.env.NODE_ENV,

    // The base directory, an absolute path, for resolving entry points and loaders from configuration
    context: path.resolve(__dirname),

    // The point or points to enter the application.
    entry: {
        dll: [
            'react',
            'react-dom',
            'monaco-editor',
            'monaco-editor/esm/vs/editor/editor.worker.js',
        ],
    },

    // Affecting the output of the compilation
    output: {
    // path: the output directory as an absolute path (required)
        path:     path.resolve(__dirname, '../build/static/js/dll/'),
        // filename: specifies the name of output file on disk (required)
        filename: 'dll.js',
        // library: name of the generated dll reference
        library:  'dll',
    },

    // A list of used webpack plugins
    plugins: [
    // Better building progress display
        new ProgressBarWebpackPlugin(),
        new MiniCssExtractPlugin({
          // Options similar to the same options in webpackOptions.output
          // both options are optional
          filename: 'static/css/[name].[contenthash:8].css',
          chunkFilename: 'static/css/[name].[contenthash:8].chunk.css',
        }),
        // Output manifest json file for each generated dll reference file
        new webpack.DllPlugin({
            path: path.resolve(__dirname, '../build/static/js/dll/manifest.json'),
            name: 'dll',
        }),
    ],

    module: {
        rules: [
            {
              oneOf: [
                // "url" loader works like "file" loader except that it embeds assets
                // smaller than specified limit in bytes as data URLs to avoid requests.
                // A missing `test` is equivalent to a match.
                {
                  test: [/\.bmp$/, /\.gif$/, /\.jpe?g$/, /\.png$/],
                  loader: require.resolve('url-loader'),
                  options: {
                    name: 'static/media/[name].[hash:8].[ext]',
                  },
                },
                // Process application JS with Babel.
                // The preset includes JSX, Flow, TypeScript, and some ESnext features.
                {
                  test: /\.(js|mjs|jsx|ts|tsx)$/,
                  include: path.resolve(__dirname, '../node_modules'),
                  loader: require.resolve('babel-loader'),
                  options: {
                    customize: require.resolve(
                      'babel-preset-react-app/webpack-overrides'
                    ),

                    plugins: [
                      [
                        require.resolve('babel-plugin-named-asset-import'),
                        {
                          loaderMap: {
                            svg: {
                              ReactComponent:
                                '@svgr/webpack?-svgo,+titleProp,+ref![path]',
                            },
                          },
                        },
                      ],
                    ],
                    // This is a feature of `babel-loader` for webpack (not Babel itself).
                    // It enables caching results in ./node_modules/.cache/babel-loader/
                    // directory for faster rebuilds.
                    cacheDirectory: true,
                    // See #6846 for context on why cacheCompression is disabled
                    cacheCompression: false,
                    compact: true,
                  },
                },
                // Process any JS outside of the app with Babel.
                // Unlike the application JS, we only compile the standard ES features.
                {
                  test: /\.(js|mjs)$/,
                  exclude: /@babel(?:\/|\\{1,2})runtime/,
                  loader: require.resolve('babel-loader'),
                  options: {
                    babelrc: false,
                    configFile: false,
                    compact: false,
                    presets: [
                      [
                        require.resolve('babel-preset-react-app/dependencies'),
                        { helpers: true },
                      ],
                    ],
                    cacheDirectory: true,
                    // See #6846 for context on why cacheCompression is disabled
                    cacheCompression: false,

                    // Babel sourcemaps are needed for debugging into node_modules
                    // code.  Without the options below, debuggers like VSCode
                    // show incorrect code and set breakpoints on the wrong lines.
                    // sourceMaps: shouldUseSourceMap,
                    // inputSourceMap: shouldUseSourceMap,
                  },
                },
                // "postcss" loader applies autoprefixer to our CSS.
                // "css" loader resolves paths in CSS and adds assets as dependencies.
                // "style" loader turns CSS into JS modules that inject <style> tags.
                // In production, we use MiniCSSExtractPlugin to extract that CSS
                // to a file, but in development "style" loader enables hot editing
                // of CSS.
                // By default we support CSS Modules with the extension .module.css
                {
                  test: /\.css$/,
                  exclude: /\.module\.css$/,
                  use: getStyleLoaders({
                    importLoaders: 1,
                    // sourceMap: isEnvProduction && shouldUseSourceMap,
                  }),
                  // Don't consider CSS imports dead code even if the
                  // containing package claims to have no side effects.
                  // Remove this when webpack adds a warning or an error for this.
                  // See https://github.com/webpack/webpack/issues/6571
                  sideEffects: true,
                },
                // Adds support for CSS Modules (https://github.com/css-modules/css-modules)
                // using the extension .module.css
                // {
                //   test: cssModuleRegex,
                //   use: getStyleLoaders({
                //     importLoaders: 1,
                //     // sourceMap: isEnvProduction && shouldUseSourceMap,
                //     modules: {
                //       getLocalIdent: getCSSModuleLocalIdent,
                //     },
                //   }),
                // },
                // Opt-in support for SASS (using .scss or .sass extensions).
                // By default we support SASS Modules with the
                // extensions .module.scss or .module.sass
                // {
                //   test: sassRegex,
                //   exclude: sassModuleRegex,
                //   use: getStyleLoaders(
                //     {
                //       importLoaders: 3,
                //       sourceMap: isEnvProduction && shouldUseSourceMap,
                //     },
                //     'sass-loader'
                //   ),
                //   // Don't consider CSS imports dead code even if the
                //   // containing package claims to have no side effects.
                //   // Remove this when webpack adds a warning or an error for this.
                //   // See https://github.com/webpack/webpack/issues/6571
                //   sideEffects: true,
                // },
                // Adds support for CSS Modules, but using SASS
                // using the extension .module.scss or .module.sass
                // {
                //   test: sassModuleRegex,
                //   use: getStyleLoaders(
                //     {
                //       importLoaders: 3,
                //       sourceMap: isEnvProduction && shouldUseSourceMap,
                //       modules: {
                //         getLocalIdent: getCSSModuleLocalIdent,
                //       },
                //     },
                //     'sass-loader'
                //   ),
                // },
                // "file" loader makes sure those assets get served by WebpackDevServer.
                // When you `import` an asset, you get its (virtual) filename.
                // In production, they would get copied to the `build` folder.
                // This loader doesn't use a "test" so it will catch all modules
                // that fall through the other loaders.
                {
                  loader: require.resolve('file-loader'),
                  // Exclude `js` files to keep "css" loader working as it injects
                  // its runtime that would otherwise be processed through "file" loader.
                  // Also exclude `html` and `json` extensions so they get processed
                  // by webpacks internal loaders.
                  exclude: [/\.(js|mjs|jsx|ts|tsx)$/, /\.html$/, /\.json$/],
                  options: {
                    name: 'static/media/[name].[hash:8].[ext]',
                  },
                },
              ]

            }
        ]
    },

    // Turn off performance hints (assets size limit)
    performance: { hints: false },
}


function getStyleLoaders(cssOptions, preProcessor) {
    const loaders = [
      {
        loader: MiniCssExtractPlugin.loader,
        // css is located in `static/css`, use '../../' to locate index.html folder
        // in production `paths.publicUrlOrPath` can be a relative path
        options: paths.publicUrlOrPath.startsWith('.')
          ? { publicPath: '../../' }
          : {},
      },
      {
        loader: require.resolve('css-loader'),
        options: cssOptions,
      },
      {
        // Options for PostCSS as we reference these options twice
        // Adds vendor prefixing based on your specified browser support in
        // package.json
        loader: require.resolve('postcss-loader'),
        options: {
          // Necessary for external CSS imports to work
          // https://github.com/facebook/create-react-app/issues/2677
          ident: 'postcss',
          plugins: () => [
            require('postcss-flexbugs-fixes'),
            require('postcss-preset-env')({
              autoprefixer: {
                flexbox: 'no-2009',
              },
              stage: 3,
            }),
            // Adds PostCSS Normalize as the reset css with default options,
            // so that it honors browserslist config in package.json
            // which in turn let's users customize the target behavior as per their needs.
            postcssNormalize(),
          ],
          // sourceMap: isEnvProduction && shouldUseSourceMap,
        },
      },
    ].filter(Boolean);
    if (preProcessor) {
      loaders.push(
        {
          loader: require.resolve('resolve-url-loader'),
          options: {
            // sourceMap: isEnvProduction && shouldUseSourceMap,
          },
        },
        {
          loader: require.resolve(preProcessor),
          options: {
            sourceMap: true,
          },
        }
      );
    }
    return loaders;
  };
