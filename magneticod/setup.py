from setuptools import find_packages, setup, Extension
import sys


def read_file(path):
    with open(path) as file:
        return file.read()


def run_setup():
    install_requirements = [
        "appdirs >= 1.4.3",
        "bencoder.pyx >= 1.1.3",
        "humanfriendly"
    ]

    if sys.platform in ["linux", "darwin"]:
        install_requirements.append("uvloop >= 0.8.0")

    setup(
        name="magneticod",
        version="0.5.0",
        description="Autonomous BitTorrent DHT crawler and metadata fetcher.",
        long_description=read_file("README.rst"),
        url="http://magnetico.org",
        author="Mert Bora ALPER",
        author_email="bora@boramalper.org",
        license="GNU Affero General Public License v3 or later (AGPLv3+)",
        packages=find_packages(),
        zip_safe=False,
        entry_points={
            "console_scripts": ["magneticod=magneticod.__main__:main"]
        },

        install_requires=install_requirements,

        classifiers=[
            "Development Status :: 2 - Pre-Alpha",
            "Environment :: No Input/Output (Daemon)",
            "Intended Audience :: End Users/Desktop",
            "License :: OSI Approved :: GNU Affero General Public License v3 or later (AGPLv3+)",
            "Natural Language :: English",
            "Operating System :: POSIX :: Linux",
            "Programming Language :: Python :: 3 :: Only",
            "Programming Language :: Python :: Implementation :: CPython",
        ],

        ext_modules=[
            Extension(
                "magneticod.bencoder._fast",
                sources=["magneticod/bencoder/_fast.c"],
            ),
        ],
    )


run_setup()
