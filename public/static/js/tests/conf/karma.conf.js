// Karma configuration
// Generated on Tue Sep 10 2013 18:10:11 GMT-0400 (EDT)

// Install karma via npm install karma -g to run

module.exports = function(config) {
  config.set({

    // base path, that will be used to resolve files and exclude
    basePath: '../../',


    // frameworks to use
    frameworks: ['jasmine'],


    // list of files / patterns to load in the browser
    files: [
      'angular.min.js',
      'angular-mocks.js',
      'angular-md5.js',
      'underscore-min.js',
      'filters/*',
      'directives/*',
      'mci_module.js',
      'build.js',
      'tests/*.js'
    ],


    // list of files to exclude
    exclude: [
    ],

    // test results reporter to use
    // possible values: 'dots', 'progress', 'junit', 'growl', 'coverage'
    reporters: ['progress'],


    // web server port
    port: 9876,


    // enable / disable colors in the output (reporters and logs)
    colors: true,


    // level of logging
    // possible values: config.LOG_DISABLE || config.LOG_ERROR || config.LOG_WARN || config.LOG_INFO || config.LOG_DEBUG
    logLevel: config.LOG_INFO,


    // enable / disable watching file and executing tests whenever any file changes
    autoWatch: true,

    plugins: [
        require( 'jasmine' ),
        require( 'karma-jasmine' ),
        require( 'karma-phantomjs-launcher' )
    ],

    // Start these browsers
    browsers: ['PhantomJS'],


    // If browser does not capture in given timeout [ms], kill it
    captureTimeout: 60000,


    // Continuous Integration mode
    // if true, it capture browsers, run tests and exit
    singleRun: true
  });
};
