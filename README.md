


# Aggregator
### A Go social media aggregator

Aggregator is a good example of how concurrency in Go works. Some of the concurrent features it implements include:

1. Concurrent downloads - limited through `ConcurrentDownloads` config. Uses a buffered go channel to ensure that no more than `ConcurrentDownloads` can run at once
2. Concurrent unzipping and saving to Redis - both of these tasks run in the same goroutine as the download runs. Only the download has limits on concurrency (unzipping and saving to Redis aren't rate-limited). With large files and fast Internet connection, unzipping can be slower than downloading, so it may be necessary to add a rate limit on unzip and Redis at some point. Otherwise, lots of unzip tasks will run concurrently and may overwhelm the CPU.

From a speed perspective, my simple concurrent implementation works fine, but for a more generic implementation with more flexible handling and further improved performance, a better concurrency model would be:

 - **download queue** - downloads files and saves to the file system or to a location in memory
 - **file queue** - handles downloaded files. For this particular aggregator, these are the types of files handled and what they each do:
	 - *html index* - parses html file for links to zip files. Links are placed onto the download queue
	 - *zip file* - extracts zip file and places uncompressed files on the file queue as each one finishes. This allows further processing to continue as soon as the first file finishes extracting (instead of the last as now).
	 - *xml file* - saves file to Redis

Once at this point, it would be easy to add different handlers for each type of file that needs to be processed. Eventually, it would make sense to make each handler into a micro-service that only knows enough to handle its particular job. By keeping everything small, simple and pluggable, this relatively simple aggregator could be built out to handle any data feed imaginable.



